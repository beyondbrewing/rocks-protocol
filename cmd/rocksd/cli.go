package main

import (
	"github.com/alecthomas/kong"
	"github.com/beyondbrewing/rocks-protocol/common/config"
)

type CLI struct {
}

func StartCli() {
	cli := CLI{}
	ctx := kong.Parse(cli,
		kong.Name(config.APP_NAME),
		kong.Description("Relay Oriented Cryptographic Key-addressed Service"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.Vars{
			"version": config.APP_VERSION,
		},
	)

	err := ctx.Run(&cli)
	ctx.FatalIfErrorf(err)
}
