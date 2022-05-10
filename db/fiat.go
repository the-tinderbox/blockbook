package db

import (
	"encoding/binary"
	"math"
	"time"

	vlq "github.com/bsm/go-vlq"
	"github.com/flier/gorocksdb"
	"github.com/golang/glog"
	"github.com/juju/errors"
)

// FiatRatesTimeFormat is a format string for storing FiatRates timestamps in rocksdb
const FiatRatesTimeFormat = "20060102150405" // YYYYMMDDhhmmss

// CurrencyRatesTicker contains coin ticker data fetched from API
type CurrencyRatesTicker struct {
	Timestamp  time.Time          // return as unix timestamp in API
	Rates      map[string]float64 // rates of the base currency against a list of vs currencies
	TokenRates map[string]float64 // rates of the tokens (identified by the address of the contract) against the base currency
}

func packTimestamp(t *time.Time) []byte {
	return []byte(t.UTC().Format(FiatRatesTimeFormat))
}

func packFloat64(buf []byte, n float64) int {
	binary.BigEndian.PutUint64(buf, math.Float64bits(n))
	return 8
}

func unpackFloat64(buf []byte) (float64, int) {
	return math.Float64frombits(binary.BigEndian.Uint64(buf)), 8
}

func packCurrencyRatesTicker(ticker *CurrencyRatesTicker) []byte {
	buf := make([]byte, 0, 32)
	varBuf := make([]byte, vlq.MaxLen64)
	l := packVaruint(uint(len(ticker.Rates)), varBuf)
	buf = append(buf, varBuf[:l]...)
	for c, v := range ticker.Rates {
		buf = append(buf, packString(c)...)
		l = packFloat64(varBuf, v)
		buf = append(buf, varBuf[:l]...)
	}
	l = packVaruint(uint(len(ticker.TokenRates)), varBuf)
	buf = append(buf, varBuf[:l]...)
	for c, v := range ticker.TokenRates {
		buf = append(buf, packString(c)...)
		l = packFloat64(varBuf, v)
		buf = append(buf, varBuf[:l]...)
	}
	return buf
}

func unpackCurrencyRatesTicker(buf []byte) (*CurrencyRatesTicker, error) {
	var (
		ticker CurrencyRatesTicker
		s      string
		l      int
		len    uint
		v      float64
	)
	len, l = unpackVaruint(buf)
	buf = buf[l:]
	if len > 0 {
		ticker.Rates = make(map[string]float64, len)
		for i := 0; i < int(len); i++ {
			s, l = unpackString(buf)
			buf = buf[l:]
			v, l = unpackFloat64(buf)
			buf = buf[l:]
			ticker.Rates[s] = v
		}
	}
	len, l = unpackVaruint(buf)
	buf = buf[l:]
	if len > 0 {
		ticker.TokenRates = make(map[string]float64, len)
		for i := 0; i < int(len); i++ {
			s, l = unpackString(buf)
			buf = buf[l:]
			v, l = unpackFloat64(buf)
			buf = buf[l:]
			ticker.TokenRates[s] = v
		}
	}
	return &ticker, nil
}

// FiatRatesConvertDate checks if the date is in correct format and returns the Time object.
// Possible formats are: YYYYMMDDhhmmss, YYYYMMDDhhmm, YYYYMMDDhh, YYYYMMDD
func FiatRatesConvertDate(date string) (*time.Time, error) {
	for format := FiatRatesTimeFormat; len(format) >= 8; format = format[:len(format)-2] {
		convertedDate, err := time.Parse(format, date)
		if err == nil {
			return &convertedDate, nil
		}
	}
	msg := "Date \"" + date + "\" does not match any of available formats. "
	msg += "Possible formats are: YYYYMMDDhhmmss, YYYYMMDDhhmm, YYYYMMDDhh, YYYYMMDD"
	return nil, errors.New(msg)
}

// FiatRatesStoreTicker stores ticker data at the specified time
func (d *RocksDB) FiatRatesStoreTicker(wb *gorocksdb.WriteBatch, ticker *CurrencyRatesTicker) error {
	if len(ticker.Rates) == 0 {
		return errors.New("Error storing ticker: empty rates")
	}
	wb.PutCF(d.cfh[cfFiatRates], packTimestamp(&ticker.Timestamp), packCurrencyRatesTicker(ticker))
	return nil
}

func getTickerFromIterator(it *gorocksdb.Iterator, token string) (*CurrencyRatesTicker, error) {
	timeObj, err := time.Parse(FiatRatesTimeFormat, string(it.Key().Data()))
	if err != nil {
		return nil, err
	}
	ticker, err := unpackCurrencyRatesTicker(it.Value().Data())
	if err != nil {
		return nil, err
	}
	if token != "" && ticker.TokenRates != nil {
		if _, found := ticker.TokenRates[token]; !found {
			return nil, nil
		}
	}
	ticker.Timestamp = timeObj.UTC()
	return ticker, nil
}

// FiatRatesFindTicker gets FiatRates data closest to the specified timestamp, of base currency or token if specified
func (d *RocksDB) FiatRatesFindTicker(tickerTime *time.Time, token string) (*CurrencyRatesTicker, error) {
	tickerTimeFormatted := tickerTime.UTC().Format(FiatRatesTimeFormat)
	it := d.db.NewIteratorCF(d.ro, d.cfh[cfFiatRates])
	defer it.Close()

	for it.Seek([]byte(tickerTimeFormatted)); it.Valid(); it.Next() {
		ticker, err := getTickerFromIterator(it, token)
		if err != nil {
			glog.Error("FiatRatesFindTicker error: ", err)
			return nil, err
		}
		if ticker != nil {
			return ticker, nil
		}
	}
	return nil, nil
}

// FiatRatesFindLastTicker gets the last FiatRates record, of base currency or token if specified
func (d *RocksDB) FiatRatesFindLastTicker(token string) (*CurrencyRatesTicker, error) {
	ticker := &CurrencyRatesTicker{}
	it := d.db.NewIteratorCF(d.ro, d.cfh[cfFiatRates])
	defer it.Close()

	for it.SeekToLast(); it.Valid(); it.Prev() {
		ticker, err := getTickerFromIterator(it, token)
		if err != nil {
			glog.Error("FiatRatesFindLastTicker error: ", err)
			return nil, err
		}
		if ticker != nil {
			return ticker, nil
		}
	}
	return ticker, nil
}
