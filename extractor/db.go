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

package extractor

import (
	"encoding/json"
	ethch "github.com/Loopring/ringminer/chainclient/eth"
	"github.com/Loopring/ringminer/config"
	"github.com/Loopring/ringminer/db"
	"github.com/Loopring/ringminer/log"
	"github.com/Loopring/ringminer/types"
	"math/big"
)

/*
该文件提供以下功能
1.保存blockhash key:blockhash,value:blockindex
2.查询blockindex
3.保存transaction key:blockhash,value:[]txhash
*/

const (
	LATEST_BLOCK_NUM            = "latestBlockNumber"
	BLOCK_HASH_TABLE_NAME       = "block_hash_table"
	TRANSACTION_HASH_TABLE_NAME = "transaction_hash_table"
)

type Rds struct {
	db             db.Database
	blockhashTable db.Database
	txhashTable    db.Database
	options        config.CommonOptions
}

//go:generate gencodec -type BlockIndex -field-override blockIndexMarshaling -out gen_blockindex_json.go
type BlockIndex struct {
	Number     *big.Int   `json:"number" 		gencodec:"required"`
	Hash       types.Hash `json:"hash"			gencodec:"required"`
	ParentHash types.Hash `json:"parentHash"	gencodec:"required"`
}

type blockIndexMarshaling struct {
	Number *types.Big
}

type TransactionIndex struct {
	Txs []types.Hash `json:"txs"	gencodec:"required"`
}

func NewRds(database db.Database, options config.CommonOptions) *Rds {
	r := &Rds{}
	r.db = database
	r.blockhashTable = db.NewTable(r.db, BLOCK_HASH_TABLE_NAME)
	r.txhashTable = db.NewTable(r.db, TRANSACTION_HASH_TABLE_NAME)
	r.options = options
	return r
}

// 存储最近一次使用的blocknumber到db，同时存储blocknumber，blockhash键值对
func (r *Rds) SaveBlock(block ethch.BlockWithTxObject) error {
	bi := createBlockIndex(block)

	// 获取最近一次使用的blockNumber
	prevBlockNum, err := r.GetBlockNumber()
	if err != nil {
		log.Infof("listener init state,non block number")
		prevBlockNum = r.options.DefaultBlockNumber
	}

	if bi.Number.Cmp(prevBlockNum) < 1 {
		log.Debugf("current block number:%s, prevent block number:%s", bi.Number.String(), prevBlockNum.String())
		// todo free comment after test
		// return errors.New("current block number cmp prevent block number < 1")
	}

	// 存储最近一次使用的blocknumber
	if err := r.SaveBlockNumber(&bi); err != nil {
		return err
	}

	// 存储blockIndex
	if err := r.SaveBlockIndex(bi); err != nil {
		return err
	}

	return nil
}

// 将最近一次使用的blockNumber存储到key LATEST_BLOCK_NUM
func (r *Rds) SaveBlockNumber(bi *BlockIndex) error {
	value, err := blockNumberToBytes(bi)
	if err != nil {
		return err
	}

	return r.db.Put([]byte(LATEST_BLOCK_NUM), value)
}

// 查询最近一次使用的blockNumber
func (r *Rds) GetBlockNumber() (*big.Int, error) {
	bs, err := r.db.Get([]byte(LATEST_BLOCK_NUM))
	if err != nil {
		return nil, err
	}
	return bytesToBlockNumber(bs)
}

// 将某个blockHash及对应的blockIndex以键值对的形式存储起来，
// 方便查询parentHash，如果另一个block的parentHash在该表找不到
// 就意味着已分叉,
func (r *Rds) SaveBlockIndex(bi BlockIndex) error {
	key := bi.Hash.Bytes()
	value, err := bi.MarshalJSON()
	if err != nil {
		return err
	}

	return r.blockhashTable.Put(key, value)
}

// 根据hash查询blockIndex信息
func (r *Rds) GetBlockIndex(key types.Hash) (*BlockIndex, error) {
	ret := &BlockIndex{}
	bs, err := r.blockhashTable.Get(key.Bytes())
	if err != nil {
		return nil, err
	}

	if err := ret.UnmarshalJSON(bs); err != nil {
		return nil, err
	}

	return ret, nil
}

// 保存block内的所有txHash
func (r *Rds) SaveTransactions(blockhash types.Hash, txhashs []types.Hash) error {
	txindex := TransactionIndex{}
	txindex.Txs = txhashs
	bs, err := json.Marshal(txindex)
	if err != nil {
		return err
	}

	return r.txhashTable.Put(blockhash.Bytes(), bs)
}

// 获取block内的所有txHash
func (r *Rds) GetTransactions(blockhash types.Hash) (*TransactionIndex, error) {
	txindex := &TransactionIndex{}
	bs, err := r.txhashTable.Get(blockhash.Bytes())
	if err != nil {
		return txindex, err
	}

	if err := json.Unmarshal(bs, txindex); err != nil {
		return nil, err
	}

	return txindex, nil
}

// 查询block内是否存在某txhash
func (r *Rds) FindTransaction(blockhash, txhash types.Hash) (bool, error) {
	bs, err := r.txhashTable.Get(blockhash.Bytes())
	if err != nil {
		return false, err
	}

	txindex := TransactionIndex{}
	if err := json.Unmarshal(bs, &txindex); err != nil {
		return false, err
	}

	for _, v := range txindex.Txs {
		if v == txhash {
			return true, nil
		}
	}

	return false, nil
}

func blockNumberToBytes(bi *BlockIndex) ([]byte, error) {
	return bi.Number.MarshalText()
}

func bytesToBlockNumber(bs []byte) (*big.Int, error) {
	n := big.NewInt(0)
	if err := n.UnmarshalText(bs); err != nil {
		return nil, err
	}
	return n, nil
}

func createBlockIndex(block ethch.BlockWithTxObject) BlockIndex {
	i := BlockIndex{}
	i.Number = block.Number.BigInt()
	i.Hash = block.Hash
	i.ParentHash = block.ParentHash

	return i
}
