/*

  Copyright 2017 Loopring Project Ltd (Loopring Foundation).

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.

*/

package gateway

import (
	"fmt"
	"github.com/Loopring/relay/config"
	"github.com/Loopring/relay/eventemiter"
	"github.com/Loopring/relay/log"
	"github.com/Loopring/relay/market/util"
	"github.com/Loopring/relay/ordermanager"
	"github.com/Loopring/relay/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type Gateway struct {
	filters          []Filter
	om               ordermanager.OrderManager
	isBroadcast      bool
	maxBroadcastTime int
	ipfsPubService   IPFSPubService
}

var gateway Gateway

type Filter interface {
	filter(o *types.Order) (bool, error)
}

func Initialize(filterOptions *config.GatewayFiltersOptions, options *config.GateWayOptions, ipfsOptions *config.IpfsOptions, om ordermanager.OrderManager) {
	// add gateway watcher
	gatewayWatcher := &eventemitter.Watcher{Concurrent: false, Handle: HandleOrder}
	eventemitter.On(eventemitter.Gateway, gatewayWatcher)

	gateway = Gateway{filters: make([]Filter, 0), om: om, isBroadcast: options.IsBroadcast, maxBroadcastTime: options.MaxBroadcastTime}
	gateway.ipfsPubService = NewIPFSPubService(ipfsOptions)

	// add filters
	baseFilter := &BaseFilter{MinLrcFee: big.NewInt(filterOptions.BaseFilter.MinLrcFee)}

	tokenFilter := &TokenFilter{AllowTokens: make(map[common.Address]bool), DeniedTokens: make(map[common.Address]bool)}
	for _, v := range util.AllTokens {
		if v.Deny {
			tokenFilter.DeniedTokens[v.Protocol] = true
		} else {
			tokenFilter.AllowTokens[v.Protocol] = true
		}
	}

	signFilter := &SignFilter{}

	//cutoffFilter := &CutoffFilter{Cache: ob.cutoffcache}

	gateway.filters = append(gateway.filters, baseFilter)
	gateway.filters = append(gateway.filters, signFilter)
	gateway.filters = append(gateway.filters, tokenFilter)
	//filters = append(filters, cutoffFilter)
}

func HandleOrder(input eventemitter.EventData) error {
	var (
		state *types.OrderState
		err   error
	)

	order := input.(*types.Order)
	order.Hash = order.GenerateHash()

	//TODO(xiaolu) 这里需要测试一下，超时error和查询数据为空的error，处理方式不应该一样
	if state, err = gateway.om.GetOrderByHash(order.Hash); err != nil {
		order.GeneratePrice()

		for _, v := range gateway.filters {
			valid, err := v.filter(order)
			if !valid {
				log.Errorf(err.Error())
				return err
			}
		}

		log.Debugf("gateway,accept new order hash:%s,amountS:%s,amountB:%s", order.Hash.Hex(), order.AmountS.String(), order.AmountB.String())

		state = &types.OrderState{}
		state.RawOrder = *order

		eventemitter.Emit(eventemitter.OrderManagerGatewayNewOrder, state)
	} else {
		return fmt.Errorf("gateway,order %s exist,will not insert again", order.Hash.Hex())
	}

	gateway.broadcast(state)
	return nil
}

func (g *Gateway) broadcast(state *types.OrderState) {
	if gateway.isBroadcast && state.BroadcastTime < gateway.maxBroadcastTime {
		//broadcast
		go func() {
			pubErr := gateway.ipfsPubService.PublishOrder(state.RawOrder)
			if pubErr != nil {
				log.Errorf("gateway,publish order %s failed", state.RawOrder.Hash.Hex())
			} else {
				gateway.om.UpdateBroadcastTimeByHash(state.RawOrder.Hash, state.BroadcastTime+1)
			}
		}()
	}
}

type BaseFilter struct {
	MinLrcFee *big.Int
}

func (f *BaseFilter) filter(o *types.Order) (bool, error) {
	const (
		addrLength = 20
		hashLength = 32
	)

	if len(o.Hash) != hashLength {
		return false, fmt.Errorf("gateway,base filter,order %s length error", o.Hash.Hex())
	}
	if len(o.TokenB) != addrLength {
		return false, fmt.Errorf("gateway,base filter,order %s tokenB %s address length error", o.Hash.Hex(), o.TokenB.Hex())
	}
	if len(o.TokenS) != addrLength {
		return false, fmt.Errorf("gateway,base filter,order %s tokenS %s address length error", o.Hash.Hex(), o.TokenS.Hex())
	}
	if o.TokenB == o.TokenS {
		return false, fmt.Errorf("gateway,base filter,order %s tokenB == tokenS", o.Hash.Hex())
	}
	if f.MinLrcFee.Cmp(o.LrcFee) >= 0 {
		return false, fmt.Errorf("gateway,base filter,order %s lrc fee %s invalid", o.Hash.Hex(), o.LrcFee.String())
	}
	if len(o.Owner) != addrLength {
		return false, fmt.Errorf("gateway,base filter,order %s owner %s address length error", o.Hash.Hex(), o.Owner.Hex())
	}
	if len(o.Protocol) != addrLength {
		return false, fmt.Errorf("gateway,base filter,order %s protocol %s address length error", o.Hash.Hex(), o.Owner.Hex())
	}
	return true, nil
}

type SignFilter struct {
}

func (f *SignFilter) filter(o *types.Order) (bool, error) {
	o.Hash = o.GenerateHash()

	if addr, err := o.SignerAddress(); nil != err {
		return false, err
	} else if addr != o.Owner {
		return false, fmt.Errorf("gateway,sign filter,o.Owner %s and signeraddress %s are not match", o.Owner.Hex(), addr.Hex())
	}

	return true, nil
}

type TokenFilter struct {
	AllowTokens  map[common.Address]bool
	DeniedTokens map[common.Address]bool
}

func (f *TokenFilter) filter(o *types.Order) (bool, error) {
	if _, ok := f.AllowTokens[o.TokenS]; !ok {
		return false, fmt.Errorf("gateway,token filter allowTokens do not contain:%s", o.TokenS.Hex())
	}
	if _, ok := f.DeniedTokens[o.TokenS]; ok {
		return false, fmt.Errorf("gateway,token filter deniedTokens contain:%s", o.TokenS.Hex())
	}
	return true, nil
}

// todo: cutoff filter

//type CutoffFilter struct {
//	Cache *CutoffIndexCache
//}
//
//// 如果订单接收在cutoff(cancel)事件之后，则该订单直接过滤
//func (f *CutoffFilter) filter(o *types.Order) (bool, error) {
//	idx, ok := f.Cache.indexMap[o.Owner]
//	if !ok {
//		return true, nil
//	}
//
//	if o.Timestamp.Cmp(idx.Cutoff) < 0 {
//		return false, errors.New("")
//	}
//
//	return true, nil
//}
