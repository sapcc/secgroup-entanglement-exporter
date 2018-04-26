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

TODO
