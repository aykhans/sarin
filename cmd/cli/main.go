package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.aykhans.me/sarin/internal/config"
	"go.aykhans.me/sarin/internal/sarin"
	"go.aykhans.me/sarin/internal/types"
	utilsErr "go.aykhans.me/utils/errors"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	go listenForTermination(func() { cancel() })

	combinedConfig := config.ReadAllConfigs()

	combinedConfig.SetDefaults()

	if *combinedConfig.ShowConfig {
		if !combinedConfig.Print() {
			return
		}
	}

	_ = utilsErr.MustHandle(combinedConfig.Validate(),
		utilsErr.OnType(func(err types.FieldValidationErrors) error {
			for _, fieldErr := range err.Errors {
				if fieldErr.Value == "" {
					fmt.Fprintln(os.Stderr,
						config.StyleYellow.Render(fmt.Sprintf("[VALIDATION] Field '%s': ", fieldErr.Field))+fieldErr.Err.Error(),
					)
				} else {
					fmt.Fprintln(os.Stderr,
						config.StyleYellow.Render(fmt.Sprintf("[VALIDATION] Field '%s' (%s): ", fieldErr.Field, fieldErr.Value))+fieldErr.Err.Error(),
					)
				}
			}
			os.Exit(1)
			return nil
		}),
	)

	srn, err := sarin.NewSarin(
		ctx,
		combinedConfig.Methods, combinedConfig.URL, *combinedConfig.Timeout,
		*combinedConfig.Concurrency, combinedConfig.Requests, combinedConfig.Duration,
		*combinedConfig.Quiet, *combinedConfig.Insecure, combinedConfig.Params, combinedConfig.Headers,
		combinedConfig.Cookies, combinedConfig.Bodies, combinedConfig.Proxies, combinedConfig.Values,
		*combinedConfig.Output != config.ConfigOutputTypeNone,
		*combinedConfig.DryRun,
		combinedConfig.Lua, combinedConfig.Js,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, config.StyleRed.Render("[ERROR] ")+err.Error())
		os.Exit(1)
	}
	_ = utilsErr.MustHandle(err,
		utilsErr.OnType(func(err types.ProxyDialError) error {
			fmt.Fprintln(os.Stderr, config.StyleRed.Render("[PROXY] ")+err.Error())
			os.Exit(1)
			return nil
		}),
	)

	srn.Start(ctx)

	switch *combinedConfig.Output {
	case config.ConfigOutputTypeNone:
		return
	case config.ConfigOutputTypeJSON:
		srn.GetResponses().PrintJSON()
	case config.ConfigOutputTypeYAML:
		srn.GetResponses().PrintYAML()
	default:
		srn.GetResponses().PrintTable()
	}
}

func listenForTermination(do func()) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	do()
}
