// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: MPL-2.0

// Package main is the entrypoint for the IPAM Terraform provider.
package main

import (
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"

	"github.com/appvia/terraform-provider-ipam/internal/provider"
)

// version is set at build time via ldflags.
var version = "dev"

const address = "registry.terraform.io/appvia/ipam"

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers")
	flag.Parse()

	var opts []tf6server.ServeOpt
	if debug {
		opts = append(opts, tf6server.WithManagedDebug())
	}

	server, err := providerserver.NewProtocol6WithError(provider.New(version)())()
	if err != nil {
		log.Fatal(err.Error())
	}

	serverFunc := func() tfprotov6.ProviderServer { return server }
	if err := tf6server.Serve(address, serverFunc, opts...); err != nil {
		log.Fatal(err.Error())
	}
}
