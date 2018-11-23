# This repo contains implmentations of several Che plugin brokers

### che-plugin-broker
Downloads tar.gz archive and:
- Evaluates Che workspace sidecars config from che-plugin.yaml
- Copies dependency files specified in dependencies.yaml

### theia-plugin-broker
Downloads .theia archive and:
- Evaluates Che workspace sidecar config for running Theia plugin as Che remote plugin in a sidecar
- Copies .theia file to /plugins/
