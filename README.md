# Overview

This repository has been a fork from <https://github.com/openshift/openshift-state-metrics>.

board-state-metrics expands upon openshift-state-metrics by adding metrics for Board specific resources.

## How to use

```
$ kubectl apply -f ./manifests # It will be deployed to board-monitoring project
```

## How to generate the manifests

You need make sure jsonnet-bundler and gojsontomal is installed, you can run this make target to install it:

```
$ make $(GOPATH)/bin/jb
$ make $(GOPATH)/bin/gojsontoyaml
```

And then  you can generate the manifests by running:

```
$ make manifests
```

## Documentation

Detailed documentation on the available metrics and usage can be found here: https://github.com/tmon546596046/board-state-metrics/blob/master/docs/README.md
