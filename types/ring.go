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

package types

import (
	"github.com/Loopring/relay/crypto"
	"github.com/Loopring/relay/log"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

// 旷工在成本节约和fee上二选一，撮合者计算出:
// 1.fee(lrc)的市场价(法币交易价格)
// 2.成本节约(savingShare)的市场价(法币交易价格)
// 撮合者在fee和savingShare中二选一，作为自己的利润，
// 如果撮合者选择fee，则成本节约分给订单发起者，如果选择成本节约，则需要返还给用户一定的lrc
// 这样一来，撮合者的利润判断公式应该是max(fee, savingShare - fee * s),s为固定比例
// 此外，在选择最优环路的时候，撮合者会在确定了选择fee/savingShare后，选择某个具有最大利润的环路
// 但是，根据谷歌竞拍法则(A出价10,B出价20,最终成交价为10)，撮合者最终获得的利润只能是利润最小的环路利润

type Ring struct {
	Orders      []*FilledOrder `json:"orderes"`
	Miner       common.Address `json:"miner"`
	V           uint8          `json:"v"`
	R           Bytes32        `json:"r"`
	S           Bytes32        `json:"s"`
	Hash        common.Hash    `json:"hash"`
	ReducedRate *big.Rat       `json:"reducedRate"` //成环之后，折价比例
	LegalFee    *big.Rat       `json:"legalFee"`    //法币计算的fee
	FeeMode     int            `json:"feeMode"`     //收费方式，0 lrc 1 share
}

func (ring *Ring) GenerateHash() common.Hash {
	vBytes := []byte{byte(ring.Orders[0].OrderState.RawOrder.V)}
	rBytes32 := ring.Orders[0].OrderState.RawOrder.R.Bytes()
	sBytes32 := ring.Orders[0].OrderState.RawOrder.S.Bytes()
	for idx, order := range ring.Orders {
		if idx > 0 {
			vBytes = Xor(vBytes, []byte{byte(order.OrderState.RawOrder.V)})
			rBytes32 = Xor(rBytes32, order.OrderState.RawOrder.R.Bytes())
			sBytes32 = Xor(sBytes32, order.OrderState.RawOrder.S.Bytes())
		}
	}
	hashBytes := crypto.GenerateHash(vBytes, rBytes32, sBytes32)
	return common.BytesToHash(hashBytes)
}

func (ring *Ring) GenerateAndSetSignature(signerAddr common.Address) error {
	if IsZeroHash(ring.Hash) {
		ring.Hash = ring.GenerateHash()
	}

	if sig, err := crypto.Sign(ring.Hash.Bytes(), signerAddr); nil != err {
		return err
	} else {
		v, r, s := crypto.SigToVRS(sig)
		ring.V = uint8(v)
		ring.R = BytesToBytes32(r)
		ring.S = BytesToBytes32(s)
		return nil
	}
}

func (ring *Ring) ValidateSignatureValues() bool {
	return crypto.ValidateSignatureValues(byte(ring.V), ring.R.Bytes(), ring.S.Bytes())
}

func (ring *Ring) SignerAddress() (common.Address, error) {
	address := &common.Address{}
	hash := ring.Hash
	if IsZeroHash(hash) {
		hash = ring.GenerateHash()
	}

	sig, _ := crypto.VRSToSig(ring.V, ring.R.Bytes(), ring.S.Bytes())
	log.Debugf("orderstate.hash:%s", hash.Hex())

	if addressBytes, err := crypto.SigToAddress(hash.Bytes(), sig); nil != err {
		log.Errorf("error:%s", err.Error())
		return *address, err
	} else {
		address.SetBytes(addressBytes)
		return *address, nil
	}
}

func (ring *Ring) GenerateSubmitArgs(miner common.Address, feeReceipt common.Address) *RingSubmitInputs {
	ringSubmitArgs := emptyRingSubmitArgs()

	for _, filledOrder := range ring.Orders {
		order := filledOrder.OrderState.RawOrder
		ringSubmitArgs.AddressList = append(ringSubmitArgs.AddressList, [2]common.Address{order.Owner, order.TokenS})
		rateAmountS, _ := new(big.Int).SetString(filledOrder.RateAmountS.FloatString(0), 10)
		ringSubmitArgs.UintArgsList = append(ringSubmitArgs.UintArgsList, [7]*big.Int{order.AmountS, order.AmountB, order.Timestamp, order.Ttl, order.Salt, order.LrcFee, rateAmountS})

		ringSubmitArgs.Uint8ArgsList = append(ringSubmitArgs.Uint8ArgsList, [2]uint8{order.MarginSplitPercentage, filledOrder.FeeSelection})

		ringSubmitArgs.BuyNoMoreThanAmountBList = append(ringSubmitArgs.BuyNoMoreThanAmountBList, order.BuyNoMoreThanAmountB)

		ringSubmitArgs.VList = append(ringSubmitArgs.VList, order.V)
		ringSubmitArgs.RList = append(ringSubmitArgs.RList, order.R)
		ringSubmitArgs.SList = append(ringSubmitArgs.SList, order.S)
	}

	if err := ring.GenerateAndSetSignature(miner); nil != err {
		log.Error(err.Error())
	} else {
		ringSubmitArgs.VList = append(ringSubmitArgs.VList, ring.V)
		ringSubmitArgs.RList = append(ringSubmitArgs.RList, ring.R)
		ringSubmitArgs.SList = append(ringSubmitArgs.SList, ring.S)
	}
	ringminer, _ := ring.SignerAddress()
	ringSubmitArgs.Ringminer = ringminer
	if IsZeroAddress(feeReceipt) {
		ringSubmitArgs.FeeRecepient = ringminer
	} else {
		ringSubmitArgs.FeeRecepient = feeReceipt
	}
	return ringSubmitArgs
}

// todo:unpack transaction data to ring,finally get orders
//type RingState struct {
//	RawRing        *Ring    `json:"rawRing"`
//	ReducedRate    *big.Rat `json:"reducedRate"` //成环之后，折价比例
//	LegalFee       *big.Rat `json:"legalFee"`    //法币计算的fee
//	FeeMode        int      `json:"feeMode"`     //收费方式，0 lrc 1 share
//}

type RingSubmitInfo struct {
	RawRing *Ring

	//todo:remove it
	Miner            common.Address
	ProtocolAddress  common.Address
	Ringhash         common.Hash
	OrdersCount      *big.Int
	ProtocolData     []byte
	ProtocolGas      *big.Int
	ProtocolGasPrice *big.Int
	RegistryData     []byte
	RegistryGas      *big.Int
	RegistryGasPrice *big.Int
	SubmitTxHash     common.Hash
	RegistryTxHash   common.Hash
	Received         *big.Rat
}

type RingSubmitInputs struct {
	AddressList              [][2]common.Address `alias:"addressList"`
	UintArgsList             [][7]*big.Int       `alias:"uintArgsList"`
	Uint8ArgsList            [][2]uint8          `alias:"uint8ArgsList"`
	BuyNoMoreThanAmountBList []bool              `alias:"buyNoMoreThanAmountBList"`
	VList                    []uint8             `alias:"vList"`
	RList                    []Bytes32           `alias:"rList"`
	SList                    []Bytes32           `alias:"sList"`
	Ringminer                common.Address      `alias:"ringminer"`
	FeeRecepient             common.Address      `alias:"feeRecepient"`
	ThrowIfLRCIsInsuffcient  bool                `alias:"throwIfLRCIsInsuffcient"`
}

func emptyRingSubmitArgs() *RingSubmitInputs {
	return &RingSubmitInputs{
		AddressList:              [][2]common.Address{},
		UintArgsList:             [][7]*big.Int{},
		Uint8ArgsList:            [][2]uint8{},
		BuyNoMoreThanAmountBList: []bool{},
		VList: []uint8{},
		RList: []Bytes32{},
		SList: []Bytes32{},
	}
}

type RingSubmitOuts struct {
}
