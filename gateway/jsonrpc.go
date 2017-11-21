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
	"context"
	"errors"
	"fmt"
	"github.com/Loopring/relay/config"
	"github.com/Loopring/relay/dao"
	"github.com/Loopring/relay/types"
	"github.com/gorilla/mux"
	gorillaRpc "github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
	"github.com/powerman/rpc-codec/jsonrpc2"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strings"
	"github.com/Loopring/relay/market"
)

func (*JsonrpcServiceImpl) Ping(val [1]string, res *string) error {
	*res = "pong for first connect, meaning server is OK"
	return nil
}

type PageResult struct {
	Data      []interface{}
	PageIndex int
	PageSize  int
	Total     int
}

var RemoteAddrContextKey = "RemoteAddr"

type JsonrpcService interface {
	Start(port string)
	Stop()
}

type JsonrpcServiceImpl struct {
	port string
	trendManager market.TrendManager
}

func NewJsonrpcService(port string, trendManager market.TrendManager) *JsonrpcServiceImpl {
	l := &JsonrpcServiceImpl{}
	l.port = port
	l.trendManager = trendManager
	return l
}

func (j *JsonrpcServiceImpl) Start2() {
	// Server export an object of type JsonrpcServiceImpl.
	rpc.Register(&JsonrpcServiceImpl{})

	// Server provide a TCP transport.
	lnTCP, err := net.Listen("tcp", "127.0.0.1:8886")
	if err != nil {
		panic(err)
	}
	defer lnTCP.Close()
	go func() {
		for {
			conn, err := lnTCP.Accept()
			if err != nil {
				return
			}
			ctx := context.WithValue(context.Background(), RemoteAddrContextKey, conn.RemoteAddr())
			go jsonrpc2.ServeConnContext(ctx, conn)
		}
	}()

	// Server provide a HTTP transport on /rpc endpoint.
	http.Handle("/rpc", jsonrpc2.HTTPHandler(nil))
	lnHTTP, err := net.Listen("tcp", ":"+j.port)
	if err != nil {
		panic(err)
	}
	defer lnHTTP.Close()
	go http.Serve(lnHTTP, nil)

	// Client use HTTP transport.
	fmt.Println(lnHTTP.Addr())
	clientHTTP := jsonrpc2.NewHTTPClient("http://" + lnHTTP.Addr().String() + "/rpc")
	defer clientHTTP.Close()

	var pong string
	err = clientHTTP.Call("JsonrpcServiceImpl.Ping", []string{"ping"}, &pong)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("ping result is %s\n", pong)
	}

}

func (j *JsonrpcServiceImpl) Start() {
	s := gorillaRpc.NewServer()
	s.RegisterCodec(json.NewCodec(), "application/json")
	s.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")
	jsonrpc := new(JsonrpcServiceImpl)
	s.RegisterService(jsonrpc, "")
	r := mux.NewRouter()
	r.Handle("/rpc", s)
	http.ListenAndServe(":"+j.port, r)
}

func (*JsonrpcServiceImpl) SubmitOrder(r *http.Request, order *types.Order, res *string) error {
	HandleOrder(order)
	*res = "SUBMIT_SUCCESS"
	return nil
}

func (*JsonrpcServiceImpl) getOrders(r *http.Request, query map[string]interface{}, res *dao.PageResult) error {

	orderQuery, pi, ps, err := convertFromMap(query)

	if err != nil {
		return err
	}

	//TODO(xiaolu) finish the connect get . not use this
	path := strings.TrimSuffix(os.Getenv("GOPATH"), "/") + "/src/github.com/Loopring/relay/config/relay.toml"
	c := config.LoadConfig(path)
	daoServiceImpl := dao.NewRdsService(c.Mysql)
	result, queryErr := daoServiceImpl.OrderPageQuery(&orderQuery, pi, ps)
	res = &result
	return queryErr
}

//TODO
func (*JsonrpcServiceImpl) getDepth(r *http.Request, market string, res *map[string]int) error {
	// not support now
	return nil
}

//TODO
func (*JsonrpcServiceImpl) getFills(r *http.Request, market string, res *map[string]int) error {
	// not support now
	return nil
}

//TODO
func (*JsonrpcServiceImpl) getTicker(r *http.Request, market string, res *map[string]int) error {
	// not support now
	return nil
}

func (j *JsonrpcServiceImpl) getTrend(r *http.Request, market string, res *[]market.Trend) error {
	trends, err := j.trendManager.GetTrends(market)
	res = &trends
	return err
}

//TODO
func (*JsonrpcServiceImpl) getRingMined(r *http.Request, market string, res *map[string]int) error {
	// not support now
	return nil
}

//TODO
func (*JsonrpcServiceImpl) getBalance(r *http.Request, market string, res *map[string]int) error {
	// not support now
	return nil
}

func convertFromMap(src map[string]interface{}) (query dao.Order, pageIndex int, pageSize int, err error) {

	for k, v := range src {
		switch k {
		//TODO(xiaolu) change status to string not uint8
		case "status":
			query.Status = v.(uint8)
		case "pageIndex":
			pageIndex = v.(int)
		case "pageSize":
			pageSize = v.(int)
		case "owner":
			query.Owner = v.(string)
		case "contractVersion":
			query.Protocol = v.(string)
		default:
			err = errors.New("unsupported query found " + k)
			return
		}
	}

	return

}

type Args struct {
	A, B int
}

type Arith int

type Result int

func (t *JsonrpcServiceImpl) Multiply(r *http.Request, args *Args, result *int) error {
	fmt.Printf("Multiplying %d with %d\n", args.A, args.B)

	*result = args.A * args.B
	return nil
}