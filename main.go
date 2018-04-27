/*******************************************************************************
*
* Copyright 2018 SAP SE
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

package main

import (
	"database/sql"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/sapcc/secgroup-entanglement-exporter/pkg/core"
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
}

func mustGetenv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		util.LogFatal("missing %s environment variable", key)
	}
	return value
}

func main() {
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

	prometheus.MustRegister(maxEntanglementGauge)
	prometheus.MustRegister(totalEntanglementGauge)
	go func() {
		for {
			collectMetrics(cfg)
			time.Sleep(5 * time.Minute)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	util.LogInfo("listening on " + cfg.ListenAddress)
	err := http.ListenAndServe(cfg.ListenAddress, nil)
	if err != nil && err != http.ErrServerClosed {
		util.LogFatal("ListenAndServe returned: " + err.Error())
	}
}

var maxEntanglementGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "security_group_max_entanglement",
		Help: "Highest entanglement score for an inter-connected set of security groups in this project.",
	},
	[]string{"project_id"},
)

var totalEntanglementGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "security_group_total_entanglement",
		Help: "Sum of entanglement scores for all inter-connected sets of security groups in this project.",
	},
	[]string{"project_id"},
)

func collectMetrics(cfg Config) {
	db, err := sql.Open("postgres", cfg.PostgresURI)
	if err != nil {
		util.LogFatal("cannot connect to Neutron DB: " + err.Error())
	}
	defer db.Close()

	projects, err := core.CollectData(db)
	if err != nil {
		util.LogFatal("cannot query Neutron DB: " + err.Error())
	}

	for projectID, project := range projects {
		var (
			maxScore   uint64
			totalScore uint64
		)

		for _, partition := range project.PartitionSecurityGroups() {
			score := partition.Score()
			totalScore += score.Value
			if maxScore < score.Value {
				maxScore = score.Value
			}

			if score.Value > cfg.ScoreLogLimit {
				partition.LogScore(score, projectID)
			}
		}

		labels := prometheus.Labels{"project_id": projectID}
		maxEntanglementGauge.With(labels).Set(float64(maxScore))
		totalEntanglementGauge.With(labels).Set(float64(totalScore))
	}
}
