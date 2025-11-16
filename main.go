// Copyright 2019 the Drone Authors. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package main

import (
	"net/http"
	"time"

	"example.com/drone-secret-1password/plugin"
	"github.com/drone/drone-go/plugin/secret"

	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
)

type spec struct {
	Bind           string        `envconfig:"DRONE_BIND"`
	Debug          bool          `envconfig:"DRONE_DEBUG"`
	Secret         string        `envconfig:"DRONE_SECRET"`
	ConnectHost    string        `envconfig:"OP_CONNECT_HOST"`
	ConnectToken   string        `envconfig:"OP_CONNECT_TOKEN"`
	ConnectTimeout time.Duration `envconfig:"OP_CONNECT_TIMEOUT" default:"15s"`
}

func main() {
	spec := new(spec)
	err := envconfig.Process("", spec)
	if err != nil {
		logrus.Fatal(err)
	}

	if spec.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	logger := logrus.StandardLogger()
	if spec.Secret == "" {
		logger.Fatalln("missing secret key")
	}
	if spec.ConnectHost == "" {
		logger.Fatalln("missing OP_CONNECT_HOST")
	}
	if spec.ConnectToken == "" {
		logger.Fatalln("missing OP_CONNECT_TOKEN")
	}
	if spec.Bind == "" {
		spec.Bind = ":3000"
	}
	if spec.ConnectTimeout == 0 {
		spec.ConnectTimeout = 15 * time.Second
	}

	client := &http.Client{Timeout: spec.ConnectTimeout}
	plug, err := plugin.New(plugin.Config{
		BaseURL:    spec.ConnectHost,
		Token:      spec.ConnectToken,
		HTTPClient: client,
		Logger:     logger,
	})
	if err != nil {
		logger.Fatal(err)
	}

	handler := secret.Handler(
		spec.Secret,
		plug,
		logger,
	)

	logger.Infof("server listening on address %s", spec.Bind)

	http.Handle("/", handler)
	logger.Fatal(http.ListenAndServe(spec.Bind, nil))
}
