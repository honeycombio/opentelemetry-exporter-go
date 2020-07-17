# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

* `withHoneycombSender` exporter option for specifying a `libhoney` `transmission.Sender`

### Changed

* The exporter now creates an isolated `libhoney.Client` instead of using the package-level api.  This reduces interactions between multiple libhoney-based instrumentations if they're run in the same process.

### Removed

* `withHoneycombOutput` exporter option.  `libhoney`'s `Output` interface is deprecated (in favor of `transmission.Sender` above) and will be removed at some point.