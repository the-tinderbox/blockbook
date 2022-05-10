//go:build unittest

package db

import (
	"reflect"
	"testing"
	"time"
)

func TestRocksTickers(t *testing.T) {
	d := setupRocksDB(t, &testBitcoinParser{
		BitcoinParser: bitcoinTestnetParser(),
	})
	defer closeAndDestroyRocksDB(t, d)

	// Test valid formats
	for _, date := range []string{"20190130", "2019013012", "201901301250", "20190130125030"} {
		_, err := FiatRatesConvertDate(date)
		if err != nil {
			t.Errorf("%v", err)
		}
	}

	// Test invalid formats
	for _, date := range []string{"01102019", "10201901", "", "abc", "20190130xxx"} {
		_, err := FiatRatesConvertDate(date)
		if err == nil {
			t.Errorf("Wrongly-formatted date \"%v\" marked as valid!", date)
		}
	}

	// Test storing & finding tickers
	key, _ := time.Parse(FiatRatesTimeFormat, "20190627000000")
	futureKey, _ := time.Parse(FiatRatesTimeFormat, "20190630000000")

	ts1, _ := time.Parse(FiatRatesTimeFormat, "20190628000000")
	ticker1 := &CurrencyRatesTicker{
		Timestamp: ts1,
		Rates: map[string]float64{
			"usd": 20000,
		},
		TokenRates: map[string]float64{
			"0x6B175474E89094C44Da98b954EedeAC495271d0F": 17.2,
		},
	}

	ts2, _ := time.Parse(FiatRatesTimeFormat, "20190629000000")
	ticker2 := &CurrencyRatesTicker{
		Timestamp: ts2,
		Rates: map[string]float64{
			"usd": 30000,
		},
		TokenRates: map[string]float64{
			"0x82dF128257A7d7556262E1AB7F1f639d9775B85E": 13.1,
			"0x6B175474E89094C44Da98b954EedeAC495271d0F": 17.5,
		},
	}
	err := d.FiatRatesStoreTicker(ticker1)
	if err != nil {
		t.Errorf("Error storing ticker! %v", err)
	}
	err = d.FiatRatesStoreTicker(ticker2)
	if err != nil {
		t.Errorf("Error storing ticker! %v", err)
	}

	ticker, err := d.FiatRatesFindTicker(&key, "") // should find the closest key (ticker1)
	if err != nil {
		t.Errorf("TestRocksTickers err: %+v", err)
	} else if ticker == nil {
		t.Errorf("Ticker not found")
	} else if ticker.Timestamp.Format(FiatRatesTimeFormat) != ticker1.Timestamp.Format(FiatRatesTimeFormat) {
		t.Errorf("Incorrect ticker found. Expected: %v, found: %+v", ticker1.Timestamp, ticker.Timestamp)
	}

	ticker, err = d.FiatRatesFindLastTicker("") // should find the last key (ticker2)
	if err != nil {
		t.Errorf("TestRocksTickers err: %+v", err)
	} else if ticker == nil {
		t.Errorf("Ticker not found")
	} else if ticker.Timestamp.Format(FiatRatesTimeFormat) != ticker2.Timestamp.Format(FiatRatesTimeFormat) {
		t.Errorf("Incorrect ticker found. Expected: %v, found: %+v", ticker1.Timestamp, ticker.Timestamp)
	}

	ticker, err = d.FiatRatesFindTicker(&futureKey, "") // should not find anything
	if err != nil {
		t.Errorf("TestRocksTickers err: %+v", err)
	} else if ticker != nil {
		t.Errorf("Ticker found, but the timestamp is older than the last ticker entry.")
	}

	ticker, err = d.FiatRatesFindTicker(&key, "0x6B175474E89094C44Da98b954EedeAC495271d0F") // should find the closest key (ticker1)
	if err != nil {
		t.Errorf("TestRocksTickers err: %+v", err)
	} else if ticker == nil {
		t.Errorf("Ticker not found")
	} else if ticker.Timestamp.Format(FiatRatesTimeFormat) != ticker1.Timestamp.Format(FiatRatesTimeFormat) {
		t.Errorf("Incorrect ticker found. Expected: %v, found: %+v", ticker1.Timestamp, ticker.Timestamp)
	}

	ticker, err = d.FiatRatesFindTicker(&key, "0x82dF128257A7d7556262E1AB7F1f639d9775B85E") // should find the last key (ticker2)
	if err != nil {
		t.Errorf("TestRocksTickers err: %+v", err)
	} else if ticker == nil {
		t.Errorf("Ticker not found")
	} else if ticker.Timestamp.Format(FiatRatesTimeFormat) != ticker2.Timestamp.Format(FiatRatesTimeFormat) {
		t.Errorf("Incorrect ticker found. Expected: %v, found: %+v", ticker1.Timestamp, ticker.Timestamp)
	}

}

func Test_packUnpackCurrencyRatesTicker(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name string
		data CurrencyRatesTicker
	}{
		{
			name: "empty",
			data: CurrencyRatesTicker{},
		},
		{
			name: "rates",
			data: CurrencyRatesTicker{
				Rates: map[string]float64{
					"usd": 2129.2341123,
					"eur": 1332.51234,
				},
			},
		},
		{
			name: "rates&tokenrates",
			data: CurrencyRatesTicker{
				Rates: map[string]float64{
					"usd": 322129.987654321,
					"eur": 291332.12345678,
				},
				TokenRates: map[string]float64{
					"0x82dF128257A7d7556262E1AB7F1f639d9775B85E": 0.4092341123,
					"0x6B175474E89094C44Da98b954EedeAC495271d0F": 12.32323232323232,
					"0xdAC17F958D2ee523a2206206994597C13D831ec7": 1332421341235.51234,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packed := packCurrencyRatesTicker(&tt.data)
			got, err := unpackCurrencyRatesTicker(packed)
			if err != nil {
				t.Errorf("unpackCurrencyRatesTicker() error = %v", err)
				return
			}
			if !reflect.DeepEqual(got, &tt.data) {
				t.Errorf("unpackCurrencyRatesTicker() = %v, want %v", *got, tt.data)
			}
		})
	}
}
