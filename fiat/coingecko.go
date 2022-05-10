package fiat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flier/gorocksdb"
	"github.com/golang/glog"
	"github.com/trezor/blockbook/db"
)

// Coingecko is a structure that implements RatesDownloaderInterface
type Coingecko struct {
	url                string
	coin               string
	platformIdentifier string
	platformVsCurrency string
	httpTimeoutSeconds time.Duration
	timeFormat         string
	httpClient         *http.Client
	db                 *db.RocksDB
}

// simpleSupportedVSCurrencies https://api.coingecko.com/api/v3/simple/supported_vs_currencies
type simpleSupportedVSCurrencies []string

type coinsListItem struct {
	ID        string            `json:"id"`
	Symbol    string            `json:"symbol"`
	Name      string            `json:"name"`
	Platforms map[string]string `json:"platforms"`
}

// coinList https://api.coingecko.com/api/v3/coins/list
type coinList []coinsListItem

// NewCoinGeckoDownloader creates a coingecko structure that implements the RatesDownloaderInterface
func NewCoinGeckoDownloader(db *db.RocksDB, url string, coin string, platformIdentifier string, platformVsCurrency string, timeFormat string) RatesDownloaderInterface {
	httpTimeoutSeconds := 15 * time.Second
	return &Coingecko{
		url:                url,
		coin:               coin,
		platformIdentifier: platformIdentifier,
		platformVsCurrency: platformVsCurrency,
		httpTimeoutSeconds: httpTimeoutSeconds,
		timeFormat:         timeFormat,
		httpClient: &http.Client{
			Timeout: httpTimeoutSeconds,
		},
		db: db,
	}
}

// doReq HTTP client
func doReq(req *http.Request, client *http.Client) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s", body)
	}
	return body, nil
}

// makeReq HTTP request helper - will retry the call after
func (cg *Coingecko) makeReq(url string) ([]byte, error) {
	for {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := doReq(req, cg.httpClient)
		if err == nil {
			return resp, err
		}
		// if there is an error, wait 1 minute and retry
		glog.Errorf("Coingecko makeReq %v error %v, will retry in 1 minute", url, err)
		time.Sleep(60 * time.Second)
	}
}

// SimpleSupportedVSCurrencies /simple/supported_vs_currencies
func (cg *Coingecko) simpleSupportedVSCurrencies() (simpleSupportedVSCurrencies, error) {
	url := cg.url + "/simple/supported_vs_currencies"
	resp, err := cg.makeReq(url)
	if err != nil {
		return nil, err
	}
	var data simpleSupportedVSCurrencies
	err = json.Unmarshal(resp, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// SimplePrice /simple/price Multiple ID and Currency (ids, vs_currencies)
func (cg *Coingecko) simplePrice(ids []string, vsCurrencies []string) (*map[string]map[string]float64, error) {
	params := url.Values{}
	idsParam := strings.Join(ids, ",")
	vsCurrenciesParam := strings.Join(vsCurrencies, ",")

	params.Add("ids", idsParam)
	params.Add("vs_currencies", vsCurrenciesParam)

	url := fmt.Sprintf("%s/simple/price?%s", cg.url, params.Encode())
	resp, err := cg.makeReq(url)
	if err != nil {
		return nil, err
	}

	t := make(map[string]map[string]float64)
	err = json.Unmarshal(resp, &t)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// CoinsList /coins/list
func (cg *Coingecko) coinsList() (coinList, error) {
	params := url.Values{}
	platform := "false"
	if cg.platformIdentifier != "" {
		platform = "true"
	}
	params.Add("include_platform", platform)
	url := fmt.Sprintf("%s/coins/list?%s", cg.url, params.Encode())
	resp, err := cg.makeReq(url)
	if err != nil {
		return nil, err
	}

	var data coinList
	err = json.Unmarshal(resp, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

var vsCurrencies []string
var platformIds []string
var platformIdsToTokens map[string]string

func (cg *Coingecko) platformIds() error {
	if cg.platformIdentifier == "" {
		return nil
	}
	cl, err := cg.coinsList()
	if err != nil {
		return err
	}
	idsMap := make(map[string]string, 64)
	ids := make([]string, 0, 64)
	for i := range cl {
		id, found := cl[i].Platforms[cg.platformIdentifier]
		if found && id != "" {
			idsMap[cl[i].ID] = id
			ids = append(ids, cl[i].ID)
		}
	}
	platformIds = ids
	platformIdsToTokens = idsMap
	return nil
}

func (cg *Coingecko) CurrentTickers() (*db.CurrencyRatesTicker, error) {
	var newTickers = db.CurrencyRatesTicker{}

	if vsCurrencies == nil {
		vs, err := cg.simpleSupportedVSCurrencies()
		if err != nil {
			return nil, err
		}
		vsCurrencies = vs
	}
	prices, err := cg.simplePrice([]string{cg.coin}, vsCurrencies)
	if err != nil || prices == nil {
		return nil, err
	}
	newTickers.Rates = make(map[string]float64, len((*prices)[cg.coin]))
	for t, v := range (*prices)[cg.coin] {
		newTickers.Rates[t] = v
	}

	if cg.platformIdentifier != "" && cg.platformVsCurrency != "" {
		if platformIdsToTokens == nil {
			err = cg.platformIds()
			if err != nil {
				return nil, err
			}
		}
		tokenPrices, err := cg.simplePrice(platformIds, []string{cg.platformVsCurrency})
		if err != nil || tokenPrices == nil {
			return nil, err
		}
		newTickers.TokenRates = make(map[string]float64, len(*tokenPrices))
		for id, v := range *tokenPrices {
			t, found := platformIdsToTokens[id]
			if found {
				newTickers.TokenRates[t] = v[cg.platformVsCurrency]
			}
		}
	}
	newTickers.Timestamp = time.Now().UTC()
	return &newTickers, nil
}

func (cg *Coingecko) UpdateHistoricalTickers() error {
	tickersToUpdate := make([]db.CurrencyRatesTicker, 0)

	// reload vs_currencies
	vs, err := cg.simpleSupportedVSCurrencies()
	if err != nil {
		return err
	}
	vsCurrencies = vs

	// update base tickers
	lastTicker, err := cg.db.FiatRatesFindLastTicker("")
	if err != nil {
		return err
	}
	if lastTicker == nil {
		lastTicker = &db.CurrencyRatesTicker{}
	}

	if cg.platformIdentifier != "" && cg.platformVsCurrency != "" {
		//  reload platform ids
		if platformIdsToTokens == nil {
			err = cg.platformIds()
			if err != nil {
				return err
			}
		}
	}

	if len(tickersToUpdate) > 0 {
		wb := gorocksdb.NewWriteBatch()
		defer wb.Destroy()
		for i := range tickersToUpdate {
			if err := cg.db.FiatRatesStoreTicker(wb, &tickersToUpdate[i]); err != nil {
				return err
			}
		}
		if err := cg.db.WriteBatch(wb); err != nil {
			return err
		}
	}
	return nil
}

// makeRequest retrieves the response from Coingecko API at the specified date.
// If timestamp is nil, it fetches the latest market data available.
func (cg *Coingecko) makeRequest(timestamp *time.Time) ([]byte, error) {
	requestURL := cg.url + "/coins/" + cg.coin
	if timestamp != nil {
		requestURL += "/history"
	}

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		glog.Errorf("Error creating a new request for %v: %v", requestURL, err)
		return nil, err
	}
	req.Close = true
	req.Header.Set("Content-Type", "application/json")

	// Add query parameters
	q := req.URL.Query()

	// Add a unix timestamp to query parameters to get uncached responses
	currentTimestamp := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	q.Add("current_timestamp", currentTimestamp)

	if timestamp == nil {
		q.Add("market_data", "true")
		q.Add("localization", "false")
		q.Add("tickers", "false")
		q.Add("community_data", "false")
		q.Add("developer_data", "false")
	} else {
		timestampFormatted := timestamp.Format(cg.timeFormat)
		q.Add("date", timestampFormatted)
	}
	req.URL.RawQuery = q.Encode()

	client := &http.Client{
		Timeout: cg.httpTimeoutSeconds,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Invalid response status: " + string(resp.Status))
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return bodyBytes, nil
}

// GetData gets fiat rates from API at the specified date and returns a CurrencyRatesTicker
// If timestamp is nil, it will download the current fiat rates.
func (cg *Coingecko) getTicker(timestamp *time.Time) (*db.CurrencyRatesTicker, error) {
	dataTimestamp := timestamp
	if timestamp == nil {
		timeNow := time.Now()
		dataTimestamp = &timeNow
	}
	dataTimestampUTC := dataTimestamp.UTC()
	ticker := &db.CurrencyRatesTicker{Timestamp: dataTimestampUTC}
	bodyBytes, err := cg.makeRequest(timestamp)
	if err != nil {
		return nil, err
	}

	type FiatRatesResponse struct {
		MarketData struct {
			Prices map[string]float64 `json:"current_price"`
		} `json:"market_data"`
	}

	var data FiatRatesResponse
	err = json.Unmarshal(bodyBytes, &data)
	if err != nil {
		glog.Errorf("Error parsing FiatRates response: %v", err)
		return nil, err
	}
	ticker.Rates = data.MarketData.Prices
	return ticker, nil
}

// MarketDataExists checks if there's data available for the specific timestamp.
func (cg *Coingecko) marketDataExists(timestamp *time.Time) (bool, error) {
	resp, err := cg.makeRequest(timestamp)
	if err != nil {
		glog.Error("Error getting market data: ", err)
		return false, err
	}
	type FiatRatesResponse struct {
		MarketData struct {
			Prices map[string]interface{} `json:"current_price"`
		} `json:"market_data"`
	}
	var data FiatRatesResponse
	err = json.Unmarshal(resp, &data)
	if err != nil {
		glog.Errorf("Error parsing Coingecko response: %v", err)
		return false, err
	}
	return len(data.MarketData.Prices) != 0, nil
}
