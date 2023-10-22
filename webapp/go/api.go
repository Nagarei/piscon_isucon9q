package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const (
	IsucariAPIToken = "Bearer 75ugk2m37a750fwir5xr-22l6h4wmue1bwrubzwd0"

	userAgent = "isucon9-qualify-webapp"
)

type APIPaymentServiceTokenReq struct {
	ShopID string `json:"shop_id"`
	Token  string `json:"token"`
	APIKey string `json:"api_key"`
	Price  int    `json:"price"`
}

type APIPaymentServiceTokenRes struct {
	Status string `json:"status"`
}

type APIShipmentCreateReq struct {
	ToAddress   string `json:"to_address"`
	ToName      string `json:"to_name"`
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
}

type APIShipmentCreateRes struct {
	ReserveID   string `json:"reserve_id"`
	ReserveTime int64  `json:"reserve_time"`
}

type APIShipmentRequestReq struct {
	ReserveID string `json:"reserve_id"`
}

type APIShipmentStatusRes struct {
	Status      string `json:"status"`
	ReserveTime int64  `json:"reserve_time"`
}
type APIShipmentStatusCache struct {
	StatusIdx   int32
	ReserveTime int64
}

type APIShipmentStatusReq struct {
	ReserveID string `json:"reserve_id"`
}

var httpclient *http.Client

func init() {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConnsPerHost = 1000
	t.MaxIdleConns = 0    //制限なし
	t.MaxConnsPerHost = 0 //制限なし
	httpclient = &http.Client{
		Transport: t,
		Timeout:   3 * time.Second,
	}
}

func APIPaymentToken(paymentURL string, param *APIPaymentServiceTokenReq) (*APIPaymentServiceTokenRes, error) {
	b, _ := json.Marshal(param)

	req, err := http.NewRequest(http.MethodPost, paymentURL+"/token", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/json")

	res, err := httpclient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read res.Body and the status code of the response from shipment service was not 200: %v", err)
		}
		return nil, fmt.Errorf("status code: %d; body: %s", res.StatusCode, b)
	}

	pstr := &APIPaymentServiceTokenRes{}
	err = json.NewDecoder(res.Body).Decode(pstr)
	if err != nil {
		return nil, err
	}

	return pstr, nil
}

var apiShipmentCache *sync.Map
var shipmentCacheDone chan interface{}

var statusString = [4]string{
	ShippingsStatusInitial,
	ShippingsStatusWaitPickup,
	ShippingsStatusShipping,
	ShippingsStatusDone,
}

func setupApiShipmentCache() error {
	apiShipmentCache = &sync.Map{}
	if shipmentCacheDone != nil {
		close(shipmentCacheDone)
	}
	shipmentCacheDone = make(chan interface{})
	return nil
}
func APIShipmentCreate(shipmentURL string, param *APIShipmentCreateReq) (*APIShipmentCreateRes, error) {
	b, _ := json.Marshal(param)

	req, err := http.NewRequest(http.MethodPost, shipmentURL+"/create", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", IsucariAPIToken)

	res, err := httpclient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read res.Body and the status code of the response from shipment service was not 200: %v", err)
		}
		return nil, fmt.Errorf("status code: %d; body: %s", res.StatusCode, b)
	}

	scr := &APIShipmentCreateRes{}
	err = json.NewDecoder(res.Body).Decode(&scr)
	if err != nil {
		return nil, err
	}
	apiShipmentCache.Store(scr.ReserveID, &APIShipmentStatusCache{
		StatusIdx:   0, //initial
		ReserveTime: scr.ReserveTime,
	})

	return scr, nil
}

func APIShipmentRequest(shipmentURL string, param *APIShipmentRequestReq) ([]byte, error) {
	b, _ := json.Marshal(param)

	req, err := http.NewRequest(http.MethodPost, shipmentURL+"/request", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", IsucariAPIToken)

	res, err := httpclient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read res.Body and the status code of the response from shipment service was not 200: %v", err)
		}
		return nil, fmt.Errorf("status code: %d; body: %s", res.StatusCode, b)
	}
	cache_, ok := apiShipmentCache.Load(param.ReserveID)
	if !ok {
		return nil, fmt.Errorf("apiShipmentCache error")
	}
	cache := cache_.(*APIShipmentStatusCache)
	atomic.StoreInt32(&cache.StatusIdx, 1) //wait_pickup
	go setupApiShipmentCacheUpdate(shipmentURL, param.ReserveID, cache)

	return io.ReadAll(res.Body)
}

func implCacheUpdate(cache *APIShipmentStatusCache, res *APIShipmentStatusRes) {
	var idx int32
	switch res.Status {
	case ShippingsStatusInitial:
		idx = 0
	case ShippingsStatusWaitPickup:
		idx = 1
	case ShippingsStatusShipping:
		idx = 2
	case ShippingsStatusDone:
		idx = 3
	}
	atomic.StoreInt32(&cache.StatusIdx, idx)
	atomic.StoreInt64(&cache.ReserveTime, res.ReserveTime)
}
func setupApiShipmentCacheUpdate(shipmentURL string, reserveID string, cache *APIShipmentStatusCache) {
	ticker := time.NewTicker(350 * time.Millisecond)
	for atomic.LoadInt32(&cache.StatusIdx) != 3 { //!= DONE
		select {
		case <-shipmentCacheDone:
			return
		case <-ticker.C:
		}
		go func() {
			res, err := implAPIShipmentStatus(shipmentURL, &APIShipmentStatusReq{ReserveID: reserveID})
			if err != nil {
				return
			}
			implCacheUpdate(cache, res)
		}()
	}
}
func implAPIShipmentStatus(shipmentURL string, param *APIShipmentStatusReq) (*APIShipmentStatusRes, error) {
	b, _ := json.Marshal(param)

	req, err := http.NewRequest(http.MethodGet, shipmentURL+"/status", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", IsucariAPIToken)

	res, err := httpclient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read res.Body and the status code of the response from shipment service was not 200: %v", err)
		}
		return nil, fmt.Errorf("status code: %d; body: %s", res.StatusCode, b)
	}

	ssr := &APIShipmentStatusRes{}
	err = json.NewDecoder(res.Body).Decode(&ssr)
	if err != nil {
		return nil, err
	}

	return ssr, nil
}
func APIShipmentStatus(shipmentURL string, param *APIShipmentStatusReq) (*APIShipmentStatusRes, error) {
	initCache := &APIShipmentStatusCache{StatusIdx: 0}
	if ptr, ok := apiShipmentCache.LoadOrStore(param.ReserveID, initCache); ok {
		cache := ptr.(*APIShipmentStatusCache)
		return &APIShipmentStatusRes{
			Status:      statusString[atomic.LoadInt32(&cache.StatusIdx)],
			ReserveTime: atomic.LoadInt64(&cache.ReserveTime),
		}, nil
	}

	res, err := implAPIShipmentStatus(shipmentURL, param)
	if err != nil {
		return nil, err
	}
	implCacheUpdate(initCache, res)
	if res.Status != ShippingsStatusDone {
		go setupApiShipmentCacheUpdate(shipmentURL, param.ReserveID, initCache)
	}
	return res, nil
}
