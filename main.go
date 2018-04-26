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
	"os"

	_ "github.com/lib/pq"

	"github.com/sapcc/secgroup-entanglement-exporter/pkg/core"
	"github.com/sapcc/secgroup-entanglement-exporter/pkg/util"
)

func main() {
	postgresURI := os.Getenv("POSTGRES_URI")
	if postgresURI == "" {
		util.LogFatal("missing POSTGRES_URI environment variable")
	}

	db, err := sql.Open("postgres", os.Getenv("POSTGRES_URI"))
	if err != nil {
		util.LogFatal("cannot connect to Neutron DB: " + err.Error())
	}
	defer db.Close()

	projects, err := core.CollectData(db)
	if err != nil {
		util.LogFatal("cannot query Neutron DB: " + err.Error())
	}

	for projectID, project := range projects {
		var maxScore uint64

		for _, partition := range project.PartitionSecurityGroups() {
			score := partition.Score()
			if maxScore < score.Value {
				maxScore = score.Value
			}

			if score.Value > 50 { //TODO make limit configurable
				partition.LogScore(score, projectID)
			}
		}

		//TODO: report this as metrics instead
		util.LogInfo("entanglement for project %s is %d", projectID, maxScore)
	}
}
