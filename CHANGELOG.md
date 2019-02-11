# Changelog

## [2.5.0] - 2018-11-23
### Change
- added Workers as param for amqp_kit.SubscribeInfo

##  [2.4.0] - 2019-02-11
### Add
- Consul client

##  [2.3.0] - 2019-02-08
### Change
- fixed opentracing span creation in amqp_kit

## [2.2.0]
### Add
- add tracing database
### Change
- 1.11.2 to travis.yml

##  [2.1.0] - 2019-01-30
### Change
- added publish with tracing
- added create custom tracer
- added amqp decode with trace and enpoint with trace
- removed err return in http do with tracing

## [2.0.0] - 2018-11-23
### Change
- VaultTransition interface

## [1.1.1] - 2018-11-23
- add amqp-kit close method
- fix amqp-kit data race serve() method

## [1.0.0] - 2018-11-13
### Added
- go mod support
