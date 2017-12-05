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

package dao

import (
	"github.com/Loopring/relay/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type RingMined struct {
	ID                 int    `gorm:"column:id;primary_key"`
	Protocol           string `gorm:"column:contract_address;type:varchar(42)"`
	RingIndex          []byte `gorm:"column:ring_index;type:varchar(30)"`
	RingHash           string `gorm:"column:ring_hash;type:varchar(82);unique_index"`
	TxHash             string `gorm:"column:tx_hash;type:varchar(82)"`
	Miner              string `gorm:"column:miner;type:varchar(42);"`
	FeeRecipient       string `gorm:"column:fee_recipient;type:varchar(42)"`
	IsRinghashReserved bool   `gorm:"column:is_ring_hash_reserved;"`
	BlockNumber        int64  `gorm:"column:block_number;type:bigint"`
	TotalLrcFee        []byte `gorm:"column:total_lrc_fee;type:varchar(30)"`
	Time               int64  `gorm:"column:time;type:bigint"`
	IsDeleted          bool   `gorm:"column:is_deleted"`
}

func (r *RingMined) ConvertDown(event *types.RingMinedEvent) error {
	var err error
	r.RingIndex, err = event.RingIndex.MarshalText()
	if err != nil {
		return err
	}
	r.TotalLrcFee, err = event.TotalLrcFee.MarshalText()
	if err != nil {
		return err
	}

	r.Protocol = event.ContractAddress.Hex()
	r.Miner = event.Miner.Hex()
	r.FeeRecipient = event.FeeRecipient.Hex()
	r.RingHash = event.Ringhash.Hex()
	r.TxHash = event.TxHash.Hex()
	r.IsRinghashReserved = event.IsRinghashReserved
	r.BlockNumber = event.Blocknumber.Int64()
	r.Time = event.Time.Int64()
	r.IsDeleted = event.IsDeleted

	return nil
}

func (r *RingMined) ConvertUp(event *types.RingMinedEvent) error {
	event.RingIndex = new(types.Big)
	if err := event.RingIndex.UnmarshalText(r.RingIndex); err != nil {
		return err
	}

	event.TotalLrcFee = new(types.Big)
	if err := event.TotalLrcFee.UnmarshalText(r.TotalLrcFee); err != nil {
		return err
	}

	event.Ringhash = common.HexToHash(r.RingHash)
	event.TxHash = common.HexToHash(r.TxHash)
	event.Miner = common.HexToAddress(r.Miner)
	event.FeeRecipient = common.HexToAddress(r.FeeRecipient)
	event.IsRinghashReserved = r.IsRinghashReserved
	event.Blocknumber = types.NewBigPtr(big.NewInt(r.BlockNumber))
	event.Time = types.NewBigPtr(big.NewInt(r.Time))
	event.IsDeleted = r.IsDeleted

	return nil
}

func (s *RdsServiceImpl) FindRingMinedByRingHash(ringHash string) (*RingMined, error) {
	var (
		model RingMined
		err   error
	)

	err = s.db.Where("ring_hash = ? and is_deleted = false", ringHash).First(&model).Error

	return &model, err
}

func (s *RdsServiceImpl) RollBackRingMined(from, to int64) error {
	err := s.db.Model(&RingMined{}).Where("block_number > ? and block_number <= ?", from, to).UpdateColumn("is_deleted", true).Error

	return err
}

func (s *RdsServiceImpl) RingMinedPageQuery(query map[string]interface{}, pageIndex, pageSize int) (res PageResult, err error) {
	ringMined := make([]RingMined, 0)
	res = PageResult{PageIndex: pageIndex, PageSize: pageSize, Data: make([]interface{}, 0)}
	err = s.db.Where(query).Order("time desc").Offset(pageIndex - 1).Limit(pageSize).Find(&ringMined).Error
	if err != nil {
		return res, err
	}
	err = s.db.Model(&RingMined{}).Where(query).Count(&res.Total).Error
	if err != nil {
		return res, err
	}

	for _, rm := range ringMined {
		res.Data = append(res.Data, rm)
	}
	return
}
