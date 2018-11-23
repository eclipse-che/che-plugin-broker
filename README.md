# This repo contains implmentations of several Che plugin brokers

### che-plugin-broker
Downloads tar.gz archive and:
- Cleanups content of /plugins/ folder
- Evaluates Che workspace sidecars config from che-plugin.yaml and data from config.json
that are placed in workdir or other path if a corresponding broker argument is used.
It contains data about Che plugin or editor from meta.yaml
- Copies dependency file/folder specified in dependencies.yaml inside of a plugin archive

### theia-plugin-broker
Downloads .theia archive and:
- Unzip it to a temp folder
- Check content of package.json file in it. If it contains {"engines.cheRuntimeContainer"} 
then this value is taken as container image for sidecar of a remote plugin. If it is missing or empty
plugin is considered non-remote
- Copies .theia file to /plugins/ for a non-remote plugin case
- Copies unzipped .theia to /plugins/ for a remote plugin case
- Sends following sidecar config to Che workspace master:
 - with plojects volume
 - with plugin volume
 - adds an endpoint with random port between 4000 and 6000 and name `port<port>`
 - adds env var to workspace-wide env vars with name 
 `THEIA_PLUGIN_REMOTE_ENDPOINT_<plugin_publisher_and_name from package.json>` and value
 `ws://port<port>:<port>`
 - adds env var to sidecar env vars with name 
 `THEIA_PLUGIN_ENDPOINT_PORT` and value `port`
- Evaluates Che workspace sidecar config for running Theia plugin as Che remote plugin in a sidecar

