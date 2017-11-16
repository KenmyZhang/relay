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
	"github.com/Loopring/relay/params"
	"gopkg.in/urfave/cli.v1"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

func main() {
	app := newApp()
	app.Action = minerNode
	app.HideVersion = true // we have a command to print the version
	app.Copyright = "Copyright 2013-2017 The Loopring Authors"
	globalFlags := GlobalFlags()
	minerFlags := MinerFlags()
	//todo:need to group flags
	app.Flags = append(app.Flags, globalFlags...)
	app.Flags = append(app.Flags, minerFlags...)

	app.Commands = []cli.Command{
		accountCommands(),
	}

	sort.Sort(cli.CommandsByName(app.Commands))

	app.Before = func(ctx *cli.Context) error {
		runtime.GOMAXPROCS(runtime.NumCPU())
		return nil
	}

	app.After = func(ctx *cli.Context) error {
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

}

func newApp() *cli.App {
	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Version = params.Version
	app.Usage = "the Loopring/relay command line interface"
	app.Author = ""
	app.Email = ""
	return app
}
