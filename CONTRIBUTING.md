### Development flow

You can do che-plugin-broker development inside Eclipse CHE workspace.

To install Eclipse CHE you need to choose infrastructure(openshift, kubernetes)
and [set up it](https://www.eclipse.org/che/docs/che-7/che-quick-starts.html#setting-up-a-local-kubernetes-or-openshift-cluster).
To create development Eclipse CHE workspace we provide che-plugin-broker devfile [devfile.yaml](devfile.yaml).
> See more about [devfile](https://redhat-developer.github.io/devfile)

### Create Eclipse CHE workspace from devfile

To start Eclipse CHE workspace, [install the latest chectl](https://www.eclipse.org/che/docs/che-7/che-quick-starts.html#installing-the-chectl-management-tool) and start new workspace from devfile:

```shell
$ chectl workspace:start --devfile=https://raw.githubusercontent.com/eclipse/che-plugin-broker/master/devfile.yaml
```

Open link to the workspace. After workspace start Eclipse CHE editor
clones che-plugin-broker source code to the folder `/projects/src/github.com/eclipse/che-plugin-broker`.
There are two development linux containers inside workspace: `dev` and `plugin-registry`.

### Dev container target

`dev` container created for development che-plugin-broker. It contains pre-installed development binaries: 
golang, dep tool, git, golangci-lint and so on. In the devfile mounted  volume `/plugins` to the `dev` container 
to store plugins binaries downloaded with help of `unified` plugin broker.

### Plugin registry container target

`plugin-registry` container it's micro-service to serve Eclipse CHE plugins meta.yaml definitions. 
devfile defines this container in the workspace like plugin-registry service exposed in the internal container's network.
`unified` plugin broker can connect to this service to get plugins meta.yaml information.

### Development commands

devfile.yaml provides development `tasks` for Eclipse CHE workspace.
List development tasks defined in the devfile `commands` section.

To launch development commands in the Eclipse CHE workspace there are three ways:

1. `My Workspace` panel. In this panel you can find development tasks and launch them by click.

2. `Run task...` menu: click `Terminal` menu in the main toolbar => click `Run task...` menu => select task by name and click it.
> Notice: also you can find menu `Run task...` using command palette. Type shortcut `Ctrl/Cmd + Shift + P` to call command palette, type `run task`.

3. Manually type task content in the terminal: `Terminal` => `Open Terminal in specific container` => select container with name `dev` and click Enter.
> Notice: use correct working dir for commands in the terminal.

### Compilation plugin brokers

There are two plugin brokers, that's why we have two commands to compile each of them.

To compile `init` plugin broker use task with name `compile "init" plugin broker`. Compiled binary will be located in the [init-plugin-broker binary folder](brokers/init/cmd).

To compile `unified` plugin broker use task with name `compile "unified" plugin broker`. Compiled binary will be located in the [unified-plugin-broker binary folder](brokers/unified/cmd).

### Start 'init' plugin broker

To start `init` plugin broker use command `start "init" plugin broker`. After execution this task volume `/plugins` in the `dev` container should be clean
(you can check it with help of terminal).

### Generate config.json with list plugins for 'unified' plugin broker

`unified` plugin broker uses config.json to get "workspace" plugin list. Then broker uses this 'list' to get meta.yaml information from plugin-registry service and download plugin binaries to the folder `plugins/sidecars`.

We provide simple task to generate sample config.json file in the same folder with `unified` plugin broker binary. Use task `generate test config.json with "list workspace" plugins for "unified" plugin broker` to invoke it.

### Start 'unified' plugin broker

Before start `unified` plugin broker first [generate config.json with list plugins](generate_config.json_with_list_plugins_for_'unified'_plugin_broker). Then use task `start "unified" plugin broker`. After execution this task volume `/plugins` should contains downloaded plugin binaries in the subfolder `sidecars`(you can check it with help of terminal).

### Run tests

To launch che-plugin-broker tests use task with name `run tests`.

### Format code

During development don't forget to format code.
To format che-plugin-broker code use task with name `format code`.

### Lint code

To lint che-plugin-broker code use task `lint code`.

### Update golang dependencies

To manage che-plugin-broker golang dependencies we are using [dep tool](https://golang.github.io/dep).
List dependencies stored in the [Gopkg.toml](Gopkg.toml). To change dependencies you need modify this file.
Use task with name `update dependencies` to flash Gopkg.toml changes:
this task call dep tool to synchronize `vendor` folder and [Gopkg.lock](Gopkg.lock) with updated list dependencies.

> Notice: `Gopkg.lock and vendor folder` changes should be contributed too.

