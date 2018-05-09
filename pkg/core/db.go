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

import "database/sql"

//Project contains all the data we collect about a project.
type Project struct {
	UUID   string
	Groups map[string]*SecurityGroup
}

//SecurityGroup contains all the data we collect about a security group.
type SecurityGroup struct {
	Name      string
	PortCount uint64
	//How many ports are shared with another security group (key = remote group name).
	SharedPortCount map[string]uint64
	//How many remote rules referencing another security group this group contains (key = remote group name).
	ReferenceCount map[string]uint64
}

var securityGroupsQuery = `
	SELECT g.tenant_id, g.name, COUNT(b.port_id)
	  FROM securitygroups g
	  JOIN securitygroupportbindings b ON b.security_group_id = g.id
	 GROUP BY g.tenant_id, g.name;
`

var sharedPortsQuery = `
	SELECT COUNT(b1.port_id),
		(SELECT name FROM securitygroups WHERE id = b1.security_group_id),
		(SELECT name FROM securitygroups WHERE id = b2.security_group_id),
		(SELECT tenant_id FROM securitygroups WHERE id = b1.security_group_id)
	  FROM securitygroupportbindings b1
	  JOIN securitygroupportbindings b2 ON b1.port_id = b2.port_id AND b1.security_group_id < b2.security_group_id
	 GROUP BY b1.security_group_id, b2.security_group_id;
`

var remoteReferencesQuery = `
	SELECT g1.tenant_id, g1.name, g2.name, COUNT(*)
	  FROM securitygrouprules r
	  JOIN securitygroups g1 ON g1.id = r.security_group_id
	  JOIN securitygroups g2 ON g2.id = r.remote_group_id
	 WHERE r.remote_group_id IS NOT NULL
	 GROUP BY g1.tenant_id, g1.name, g2.name;
`

//CollectData gathers data about all security groups in all projects from the Neutron DB.
func CollectData(db *sql.DB) (map[string]*Project, error) {
	result := make(map[string]*Project)

	//list all security groups in all projects
	var (
		projectID string
		groupName string
		portCount uint64
	)
	err := scan(db, securityGroupsQuery, args(&projectID, &groupName, &portCount), func() {
		project, exists := result[projectID]
		if !exists {
			project = &Project{projectID, make(map[string]*SecurityGroup)}
			result[projectID] = project
		}
		project.Groups[groupName] = &SecurityGroup{
			Name:            groupName,
			PortCount:       portCount,
			SharedPortCount: make(map[string]uint64),
			ReferenceCount:  make(map[string]uint64),
		}
	})
	if err != nil {
		return nil, err
	}

	//count ports shared by multiple security groups
	var (
		groupName1 string
		groupName2 string
	)
	err = scan(db, sharedPortsQuery, args(&portCount, &groupName1, &groupName2, &projectID), func() {
		//This is coded defensively, but if the Neutron DB is consistent *cough*,
		//we should never have `exists && exists1 && exists2 = false`
		if project, exists := result[projectID]; exists {
			group1, exists1 := project.Groups[groupName1]
			group2, exists2 := project.Groups[groupName2]
			if exists1 && exists2 {
				group1.SharedPortCount[groupName2] = portCount
				group2.SharedPortCount[groupName1] = portCount
			}
		}
	})
	if err != nil {
		return nil, err
	}

	//find security groups with rules referencing other security groups
	var (
		remoteGroupName string
		referenceCount  uint64
	)
	err = scan(db, remoteReferencesQuery, args(&projectID, &groupName, &remoteGroupName, &referenceCount), func() {
		//This is coded defensively, see above.
		if project, exists := result[projectID]; exists {
			group, exists := project.Groups[groupName]
			_, remoteExists := project.Groups[remoteGroupName]
			if exists && remoteExists {
				group.ReferenceCount[remoteGroupName] = referenceCount
			}
		}
	})

	return result, err
}

func scan(db *sql.DB, query string, args []interface{}, action func()) error {
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	for rows.Next() {
		err := rows.Scan(args...)
		if err != nil {
			return err
		}
		action()
	}
	return nil
}

//Syntactic sugar for scan().
func args(vals ...interface{}) []interface{} {
	return vals
}
