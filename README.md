## Go-Utils. Reusable GoLang utils.

[![Build Status](https://travis-ci.org/space307/go-utils.svg?branch=master)](https://travis-ci.org/space307/go-utils)

### Table of Contents
1. [debug.go](#debug)
2. [grace.go](#grace)
3. [ticker.go](#ticker)
4. [config.go](#config)
5. [api.go](#api)
6. [server_group.go](#sg)
7. [tracing.go](#tracing)
8. [checker.go](#checker)
9. [vault.go](#vault)
10. [json_formatter.go](#formatter)
11. [messagebus.go](#messagebus)

<a name="debug" />

### 1. debug

Starting Pprof Server to a free port from a specified range.  Prints all goroutine stacks to stdout

<a name="grace" />

### 2. grace

Package processes the low-level operating system call : syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP .

<a name="ticker" />

### 3. ticker

Creates a new ticker, which sends current time to its channel every second, minute, or hour after the specified delay.
Wrap over standard time

<a name="config" />

### 4. config

A set of basic utilities for working with config files

<a name="api" />

### 5. api

A simple REST API server based on gorilla mux, go-kit handlers, and a standard http.Server.

<a name="sg" />

### 6. sg

ServerGroup allows to start several sg.Server objects and stop them, tracking errors.

<a name="tracing" />

### 7. tracing

HTTP client based on standart [net/http](https://golang.org/pkg/net/http/) client with additional method adding a Zipkin span to request.

<a name="checker" />

### 8. checker

Helper for execute series of tests

<a name="vault" />

### 9. vault

Helper for working with vault hashicorp

<a name="formatter" />

### 10. formatter

Implementation JSONFormatter of [logrus](https://github.com/sirupsen/logrus) with supporting additional fields for output

<a name="messagebus" />

### 11. messagebus

Handy wrapper for [amqp](https://github.com/streadway/amqp)
