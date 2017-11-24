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

package main

import (
	"fmt"
	"github.com/Loopring/relay/config"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/urfave/cli.v1"
	"reflect"
	"syscall"
)

func GlobalFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:  "config,c",
			Usage: "config file",
		},
		cli.StringFlag{
			Name:  "unlock",
			Usage: "the list of accounts to unlock",
		},
		cli.StringFlag{
			Name:  "pwdfile",
			Usage: "the file contains passwords used to unlock accounts ",
		},
	}
}

func MinerFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:  "ringMaxLength,rml",
			Usage: "the max length of ring",
		},
		cli.StringFlag{
			Name:  "miner",
			Usage: "the encrypted private key used to sign ring",
		},
		cli.StringFlag{
			Name:  "feeRecepient,r",
			Usage: "the fee recepient address when mined a ring",
		},
		cli.BoolFlag{
			Name:  "ifRegistryRingHash,reg",
			Usage: "the submitter will registry ringhash first if it set ture",
		},
		cli.BoolFlag{
			Name:  "throwIfLrcIsInsuffcient,t",
			Usage: "the contract will revert when the lrc is insuffcient if it set ture ",
		},
	}
}

func mergeMinerConfig(ctx *cli.Context, minerOpts *config.MinerOptions) {
	if ctx.IsSet("ringMaxLength") {
		minerOpts.RingMaxLength = ctx.Int("ringMaxLength")
	}
	if ctx.IsSet("miner") {
		minerOpts.Miner = ctx.String("miner")
	}
	if ctx.IsSet("feeRecepient") {
		minerOpts.FeeRecepient = ctx.String("feeRecepient")
	}
	if ctx.IsSet("ifRegistryRingHash") {
		minerOpts.IfRegistryRingHash = ctx.Bool("ifRegistryRingHash")
	}
	if ctx.IsSet("throwIfLrcIsInsuffcient") {
		minerOpts.ThrowIfLrcIsInsuffcient = ctx.Bool("throwIfLrcIsInsuffcient")
	}
}

func setGlobalConfig(ctx *cli.Context) *config.GlobalConfig {
	file := ""
	if ctx.IsSet("config") {
		file = ctx.String("config")
	}
	globalConfig := config.LoadConfig(file)
	mergeMinerConfig(ctx, &globalConfig.Miner)

	if _, err := config.Validator(reflect.ValueOf(globalConfig).Elem()); nil != err {
		panic(err)
	}

	return globalConfig
}

func passphraseFromCtx(ctx *cli.Context, tip string) string {
	if ctx.IsSet("passphrase") {
		return ctx.String("passphrase")
	} else {
		if "" == tip {
			tip = "enter passphrase："
		}
		fmt.Print(tip)
		if passphrase, err := terminal.ReadPassword(int(syscall.Stdin)); nil != err {
			panic(err)
		} else {
			return passphrase
		}
	}
}
