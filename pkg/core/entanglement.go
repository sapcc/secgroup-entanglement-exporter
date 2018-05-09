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

package core

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sapcc/secgroup-entanglement-exporter/pkg/util"
)

//Partition is a set of interconnected security groups (all groups form a
//connected graph either via remote references in a security group rule, or by
//ports that are in multiple security groups). The map key is the group's name.
type Partition map[string]*SecurityGroup

//Factor is an aspect of a Partition's topology that contributes to its
//entanglement score.
type Factor struct {
	Value  uint64
	Reason string
}

//Score is the entanglement score of a partition.
type Score struct {
	Value   uint64
	Factors []Factor
}

//PartitionSecurityGroups separate the security groups in this project into
//Partitions.
func (p Project) PartitionSecurityGroups() (result []Partition) {
	partitioned := make(map[string]bool)
	for {
		partition := make(map[string]*SecurityGroup)

		//when adding a group to the partition, also add all connected groups
		var addRecursively func(string)
		addRecursively = func(groupName string) {
			group := p.Groups[groupName]
			partition[groupName] = group
			partitioned[groupName] = true //do not consider this group for future partitions

			for _, otherGroup := range p.Groups {
				if group.SharedPortCount[otherGroup.Name] > 0 || group.ReferenceCount[otherGroup.Name] > 0 {
					if !partitioned[otherGroup.Name] {
						addRecursively(otherGroup.Name)
					}
				}
			}
		}

		//pick a security group at random to start this partition
		for groupName := range p.Groups {
			if !partitioned[groupName] {
				addRecursively(groupName)
				break
			}
		}

		if len(partition) > 0 {
			result = append(result, partition)
		} else {
			//no more security groups left to partition
			return result
		}
	}
}

//Score returns this partition's entanglement score.
func (groups Partition) Score() (result Score) {
	sharedGroupCount := uint64(0)
	for _, group := range groups {
		for _, portCount := range group.SharedPortCount {
			if portCount > 0 {
				sharedGroupCount++
			}
		}
	}
	//we double-counted because groups["X"].SharedPortCount["Y"] == groups["Y"].SharedPortCount["X"]
	sharedGroupCount /= 2
	if sharedGroupCount > 0 {
		result.Factors = append(result.Factors, Factor{
			Value: sharedGroupCount,
			Reason: fmt.Sprintf(
				"%d pairs of security groups are shared by ports",
				sharedGroupCount,
			),
		})
	}

	for groupName, group := range groups {
		for otherGroupName, otherGroup := range groups {
			if group.ReferenceCount[otherGroup.Name] > 0 && otherGroup.PortCount > 0 {
				result.Factors = append(result.Factors, Factor{
					Value: otherGroup.PortCount * group.ReferenceCount[otherGroup.Name],
					Reason: fmt.Sprintf(
						"security group %s has %d rules referencing security group %s which contains %d ports",
						groupName, group.ReferenceCount[otherGroup.Name], otherGroupName, otherGroup.PortCount,
					),
				})
			}
		}
	}

	result.Value = 0
	for _, factor := range result.Factors {
		result.Value += factor.Value
	}
	return result
}

//LogScore produces a log message for this partition's entanglement score.
func (groups Partition) LogScore(score Score, projectID string) {
	names := make([]string, 0, len(groups))
	for groupName := range groups {
		names = append(names, groupName)
	}
	sort.Strings(names)

	//sort scores descending by value
	sort.Slice(score.Factors, func(i, j int) bool {
		return score.Factors[i].Value > score.Factors[j].Value
	})

	//report top 3 scores contributing to this partition's total score
	reasons := make([]string, 0, 3)
	for _, factor := range score.Factors {
		reasons = append(reasons, factor.Reason)
		if len(reasons) == 3 {
			break
		}
	}

	util.LogInfo(
		"project %s contains a partition of %d security groups (%s) with entanglement %d; top %d reasons: %s",
		projectID,
		len(groups),
		strings.Join(names, ", "),
		score.Value,
		len(reasons),
		strings.Join(reasons, ", "),
	)
}
