# tilt-apiserver

[![Build Status](https://circleci.com/gh/tilt-dev/tilt-apiserver/tree/main.svg?style=shield)](https://circleci.com/gh/tilt-dev/tilt-apiserver)
[![GoDoc](https://godoc.org/github.com/tilt-dev/tilt-apiserver?status.svg)](https://pkg.go.dev/github.com/tilt-dev/tilt-apiserver)

## Why

Tilt is a toolkit for building multi-service dev environments.

Tilt offers many first-party data types like Docker builds and Kubernetes
appliers. But long-term, we want to make it easier for users to define their own
data types and behaviors.

The future of Tilt is a simple model consisting of very few types of
building blocks, and a mix of uniformity and versatility whereby using the same
simple elements one can build complex systems and different types of
functionality.

The Tilt apiserver is the base layer of that model.

## What

The Tilt apiserver is a full-fledged Kubernetes API server. You can query it
with `kubectl` or any standard Kubernetes tooling.

This repo offers:

- A builder for registering data types, storing them in memory, and serving them
  up on any port on HTTP
- An example data type
- An example of generated client code

This repo is intended primarily for consumption by https://github.com/tilt-dev/tilt,
where we define the first-party data types and controllers that Tilt needs.

## How

To develop the API server, install [Tilt](https://tilt.dev/) and run:

```
tilt up
```

This will present you with a list of common commands we run in development.

You may prefer running commands in the terminal.

To run on port 9443:

```
make run-apiserver
```

To create a new data object:

```
kubectl --kubeconfig kubeconfig apply -f manifest.yaml
```

## Credits

Big thanks to the Kubernetes community for all the help and documentation on how to
use their API infrastructure. We could not have built this without inspiration from:

- https://github.com/kubernetes/sample-apiserver
- https://github.com/kubernetes-sigs/apiserver-builder-alpha
- https://github.com/kubernetes-sigs/apiserver-runtime
- https://github.com/thetirefire/badidea

## License

Copyright 2020 The Tilt Dev Authors

Licensed under [the Apache License, Version 2.0](LICENSE)

