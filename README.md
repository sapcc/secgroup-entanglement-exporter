# ARCHIVED

We don't use VMware DVS anymore, so there is no need for this exporter anymore.

# Security group entanglement exporter

At SAP, we use VMware DVS to apply security groups to VMware instances. Certain
security group setups with a lot of connections between security groups put a
lot of load on the neutron-dvs-agent. To detect and handle such situations,
this exporter queries the Neutron database, finds interconnected sets of
security groups, and calculates their **entanglement score**. The highest
entanglement score in a project is exported as a Prometheus gauge.

## Building and running

Build and install with `make` and `make install`, or produce an image with `docker build`.

## Entanglement: What it means and how it's computed

Suppose we have a project with the following security groups:

- group `default` with rules:
  - allow SSH from group `jumpservers`
- group `jumpservers` with rules:
  - allow SSH from 0.0.0.0/0
- group `database` with rules:
  - allow PostgreSQL from group `appservers`
- group `appservers` with no special rules

Also, suppose that the project contains the following instances:

- 2 jump servers in the security group `jumpservers`
- 10 app servers in the security groups `appservers` and `default`
- 1 DB server in the security groups `database` and `default`

This leads to the following entanglement graph:

![Entanglement graph](./doc/entanglement-graph1.svg)

The nodes in this graph are the security groups, and there are two kinds of edges:

1. There is a dashed edge between each pair of security groups that is shared by at least one instance. For example, the DB server is in the security groups `database` and `default`, so there is a dashed edge between these two groups.
2. There is a solid edge for each security group rule that references a remote security group, pointing from the group containing the rule to the remote group it references.

The **entanglement score** is computed as follows:

1. Each dashed edge adds one to the score (because sharing of security groups increases the number of port groups in the DVS).
2. Each solid edge adds the number of ports in the remote group to the score (because each time ports are added to or removed from the remote group, all port groups using this remote security group need to be updated by the DVS agent).

As you can see, the entanglement score is a measure for the amount of work imposed on the DVS agent by this particular setup of security groups. The lower, the better. In this example, we have

```
entanglement score
  = 2 (dashed edges)
  + 2 (number of instances in the jumpservers group referenced by the default group)
  + 10 (number of instances in the appservers group referenced by the database group)
  = 14
```

This is pretty high for such a small project, so we should try to bring this down. Since the last term is the largest one, we should get rid of the reference from `database` to `appservers`. This can be done by placing all app servers in a separate subnet. When the `database` security group is amended to reference that subnet instead of the `appservers` security group, the entanglement score drops from 14 to 4.

In larger projects, the entanglement graph may not be fully connected. In this case, the entanglement score is calculated separately for each maximal connected subgraph of the entanglement graph. The `security_group_max_entanglement` metric reports the highest of these subscores, and the `security_group_total_entanglement` metric is the sum of all subscores.
