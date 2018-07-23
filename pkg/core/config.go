/******************************************************************************
*
*  Copyright 2018 Stefan Majewsky <majewsky@gmx.net>
*
*  Licensed under the Apache License, Version 2.0 (the "License");
*  you may not use this file except in compliance with the License.
*  You may obtain a copy of the License at
*
*      http://www.apache.org/licenses/LICENSE-2.0
*
*  Unless required by applicable law or agreed to in writing, software
*  distributed under the License is distributed on an "AS IS" BASIS,
*  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*  See the License for the specific language governing permissions and
*  limitations under the License.
*
******************************************************************************/

package core

import (
	"os"
	"regexp"
	"strconv"

	"github.com/sapcc/secgroup-entanglement-exporter/pkg/util"
)

//Config contains the configuration for the exporter.
type Config struct {
	//URI for Neutron DB.
	PostgresURI string
	//Address to listen on for Prometheus metrics endpoint.
	ListenAddress string
	//Partitions with score higher than this will be logged (default: 50).
	ScoreLogLimit uint64

	//Parts of the database schema that change between Neutron versions.
	DatabaseSchema struct {
		ProjectIDColumnName string
	}
}

func mustGetenv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		util.LogFatal("missing %s environment variable", key)
	}
	return value
}

//ReadConfigFromEnv reads the supported environment variables into a Config
//object, or panics if the given configuration is broken or incomplete.
func ReadConfigFromEnv() Config {
	cfg := Config{
		PostgresURI:   mustGetenv("POSTGRES_URI"),
		ListenAddress: mustGetenv("LISTEN_ADDRESS"),
		ScoreLogLimit: 50,
	}
	if str := os.Getenv("SCORE_LOG_LIMIT"); str != "" {
		var err error
		cfg.ScoreLogLimit, err = strconv.ParseUint(str, 10, 64)
		if err != nil {
			util.LogFatal("invalid value for SCORE_LOG_LIMIT: " + err.Error())
		}
	}
	switch mustGetenv("NEUTRON_RELEASE") {
	case "kilo", "liberty", "mitaka":
		cfg.DatabaseSchema.ProjectIDColumnName = "tenant_id"
	case "newton", "ocata", "pike", "queens":
		cfg.DatabaseSchema.ProjectIDColumnName = "project_id"
	default:
		util.LogFatal("unknown value found in NEUTRON_RELEASE environment variable")
	}

	return cfg
}

var projectIDColumnNameRx = regexp.MustCompile(`\bproject_id\b`)

//Apply the variables in cfg.DatabaseSchema to the given query string.
func (cfg Config) applyTo(query string) string {
	return projectIDColumnNameRx.ReplaceAllString(query, cfg.DatabaseSchema.ProjectIDColumnName)
}
