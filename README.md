## Go-Utils. Reusable GoLang utils.

[![Build Status](https://travis-ci.org/space307/go-utils.svg?branch=master)](https://travis-ci.org/space307/go-utils)

### Table of Contents
1. [debug.go](#debug)
2. [grace.go](#grace)
3. [ticker.go](#ticker)
4. [config.go](#config)
5. [api.go](#api)
6. [server_group.go](#sg)

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

