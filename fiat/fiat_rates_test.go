//go:build unittest

package fiat

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/flier/gorocksdb"
	"github.com/golang/glog"
	"github.com/martinboehm/btcutil/chaincfg"
	"github.com/trezor/blockbook/bchain"
	"github.com/trezor/blockbook/bchain/coins/btc"
	"github.com/trezor/blockbook/common"
	"github.com/trezor/blockbook/db"
)

func TestMain(m *testing.M) {
	// set the current directory to blockbook root so that ./static/ works
	if err := os.Chdir(".."); err != nil {
		glog.Fatal("Chdir error:", err)
	}
	c := m.Run()
	chaincfg.ResetParams()
	os.Exit(c)
}

func setupRocksDB(t *testing.T, parser bchain.BlockChainParser) (*db.RocksDB, *common.InternalState, string) {
	tmp, err := ioutil.TempDir("", "testdb")
	if err != nil {
		t.Fatal(err)
	}
	d, err := db.NewRocksDB(tmp, 100000, -1, parser, nil)
	if err != nil {
		t.Fatal(err)
	}
	is, err := d.LoadInternalState("fakecoin")
	if err != nil {
		t.Fatal(err)
	}
	d.SetInternalState(is)
	return d, is, tmp
}

func closeAndDestroyRocksDB(t *testing.T, db *db.RocksDB, dbpath string) {
	// destroy db
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(dbpath)
}

type testBitcoinParser struct {
	*btc.BitcoinParser
}

func bitcoinTestnetParser() *btc.BitcoinParser {
	return btc.NewBitcoinParser(
		btc.GetChainParams("test"),
		&btc.Configuration{BlockAddressesToKeep: 1})
}

// getFiatRatesMockData reads a stub JSON response from a file and returns its content as string
func getFiatRatesMockData(name string) (string, error) {
	var filename string
	filename = "fiat/mock_data/" + name + ".json"
	mockFile, err := os.Open(filename)
	if err != nil {
		glog.Errorf("Cannot open file %v", filename)
		return "", err
	}
	b, err := ioutil.ReadAll(mockFile)
	if err != nil {
		glog.Errorf("Cannot read file %v", filename)
		return "", err
	}
	return string(b), nil
}

func TestFiatRates(t *testing.T) {
	d, _, tmp := setupRocksDB(t, &testBitcoinParser{
		BitcoinParser: bitcoinTestnetParser(),
	})
	defer closeAndDestroyRocksDB(t, d, tmp)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		var mockData string

		if r.URL.Path == "/ping" {
			w.WriteHeader(200)
		} else if r.URL.Path == "/coins/ethereum/history" {
			date := r.URL.Query()["date"][0]
			mockData, err = getFiatRatesMockData(date) // get stub rates by date
		} else if r.URL.Path == "/coins/ethereum" {
			mockData, err = getFiatRatesMockData("current") // get "latest" stub rates
		} else if r.URL.Path == "/coins/list" {
			mockData, err = getFiatRatesMockData("coinlist")
		} else if r.URL.Path == "/simple/supported_vs_currencies" {
			mockData, err = getFiatRatesMockData("vs_currencies")
		} else if r.URL.Path == "/simple/price" {
			if r.URL.Query().Get("ids") == "ethereum" {
				mockData, err = getFiatRatesMockData("simpleprice_base")
			} else {
				mockData, err = getFiatRatesMockData("simpleprice_tokens")
			}
		} else {
			t.Errorf("Unknown URL path: %v", r.URL.Path)
		}

		if err != nil {
			t.Errorf("Error loading stub data: %v", err)
		}
		fmt.Fprintln(w, mockData)
	}))
	defer mockServer.Close()

	// mocked CoinGecko API
	configJSON := `{"fiat_rates": "coingecko", "fiat_rates_params": "{\"url\": \"` + mockServer.URL + `\", \"coin\": \"ethereum\",\"platformIdentifier\":\"ethereum\",\"platformVsCurrency\": \"eth\",\"periodSeconds\": 60}"}`

	type fiatRatesConfig struct {
		FiatRates       string `json:"fiat_rates"`
		FiatRatesParams string `json:"fiat_rates_params"`
	}

	var config fiatRatesConfig
	err := json.Unmarshal([]byte(configJSON), &config)
	if err != nil {
		t.Errorf("Error parsing config: %v", err)
	}

	if config.FiatRates == "" || config.FiatRatesParams == "" {
		t.Errorf("Error parsing FiatRates config - empty parameter")
		return
	}
	testStartTime := time.Date(2019, 11, 22, 16, 0, 0, 0, time.UTC)
	fiatRates, err := NewFiatRatesDownloader(d, config.FiatRates, config.FiatRatesParams, &testStartTime, nil)
	if err != nil {
		t.Errorf("FiatRates init error: %v\n", err)
	}
	if config.FiatRates == "coingecko" {

		currentTickers, err := fiatRates.downloader.CurrentTickers()
		if err != nil {
			t.Errorf("Error in CurrentTickers: %v", err)
			return
		}
		if currentTickers == nil {
			t.Errorf("CurrentTickers returned nil value")
			return
		}

		wantCurrentTickers := db.CurrencyRatesTicker{
			Rates: map[string]float64{
				"aed": 8447.1,
				"ars": 268901,
				"aud": 3314.36,
				"btc": 0.07531005,
				"eth": 1,
				"eur": 2182.99,
				"ltc": 29.097696,
				"usd": 2299.72,
			},
			TokenRates: map[string]float64{
				"0x5e9997684d061269564f94e5d11ba6ce6fa9528c": 5.58195e-07,
				"0x906710835d1ae85275eb770f06873340ca54274b": 1.39852e-10,
			},
			Timestamp: currentTickers.Timestamp,
		}
		if !reflect.DeepEqual(currentTickers, &wantCurrentTickers) {
			t.Errorf("CurrentTickers() = %v, want %v", *currentTickers, wantCurrentTickers)
		}

		timestamp, err := fiatRates.findEarliestMarketData()
		if err != nil {
			t.Errorf("Error looking up earliest market data: %v", err)
			return
		}
		earliestTimestamp, _ := time.Parse(db.FiatRatesTimeFormat, "20130429000000")
		if *timestamp != earliestTimestamp {
			t.Errorf("Incorrect earliest available timestamp found. Wanted: %v, got: %v", earliestTimestamp, timestamp)
			return
		}

		// After verifying that findEarliestMarketData works correctly,
		// set the earliest available timestamp to 2 days ago for easier testing
		*timestamp = fiatRates.startTime.Add(time.Duration(-24*2) * time.Hour)

		err = fiatRates.syncHistorical(timestamp)
		if err != nil {
			t.Errorf("RatesDownloader syncHistorical error: %v", err)
			return
		}
		ticker, err := fiatRates.downloader.getTicker(fiatRates.startTime)
		if err != nil {
			// Do not exit on GET error, log it, wait and try again
			glog.Errorf("Sync GetData error: %v", err)
			return
		}
		wb := gorocksdb.NewWriteBatch()
		defer wb.Destroy()
		if err := fiatRates.db.FiatRatesStoreTicker(wb, ticker); err != nil {
			glog.Errorf("Sync StoreTicker error %v", err)
			return
		}
		if err := d.WriteBatch(wb); err != nil {
			glog.Errorf("Sync StoreTicker error %v", err)
			return
		}

	}
}
