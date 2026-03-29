# Dealing with Block Numbers Across Providers

There are several different ways in which a collection of providers might be considered healthy or unhealthy based on the blocknumbers of the various providers serving the same network. This document explores several of these scenarios.

## Legend:

| Symbol             | Meaning                                                                                        |
| ------------------ | ---------------------------------------------------------------------------------------------- |
| :white_check_mark: | Healthy: Traffic will be routed to this provider                                               |
| :x:                | Unhealthy: Traffic will be not routed to this provider, even if that means serving a 502       |
| :warning:          | Warning: Traffic will not be routed to this provider unless no healthy providers are available |

Healthy / Unhealthy / Warning describe health check datapoints, and we may use multiple datapoints to determine the provider's health check status. A provider that has been unhealthy may not go to healthy immediately upon registering a single healthy datapoint.

## Scenario 1

| Provider | T1 | :black_square_button: | T2 | :black_square_button: | T3 | :black_square_button: |
| -------- | -- | --------------------- | -- | --------------------- | -- | --------------------- |
| A        | 50 | :white_check_mark:    | 51 | :white_check_mark:    | 52 | :white_check_mark:    |
| B        | 52 | :white_check_mark:    | 52 | :white_check_mark:    | 53 | :white_check_mark:    |

In this scenario, provider A lags slightly behind provider B, but within the configured lag allowance, so both providers are consistently considered healthy.

## Scenario 2

| Provider | T1 | :black_square_button: | T2   | :black_square_button: |
| -------- | -- | --------------------- | ---- | --------------------- |
| A        | 50 | :white_check_mark:    | 51   | :white_check_mark:    |
| B        | 52 | :white_check_mark:    | 5000 | :x:                   |

In this scenario, provider A jumps ahead of both provider A and ahead of itself by a large number of blocks. This likely indicates that the provider is serving the wrong network, and provider B should be considered unhealthy.


## Scenario 3

| Provider | T1 | :black_square_button: | T2  | :black_square_button: | T3  | :black_square_button: |
| -------- | -- | --------------------- | --- | --------------------- | --- | --------------------- |
| A        | 99 | :white_check_mark:    | 100 | :white_check_mark:    | 101 | :white_check_mark:    |
| B        | 52 | :x:                   | 52  | :x:                   | 101 | :white_check_mark:    |

In this scenario, provider B was significantly behind provider A, and thus considered unhealthy. Provider B leapt forward to match provider A, and becomes healthy upon catching up.

## Scenario 4

| Provider | T1 | :black_square_button: | T2  | :black_square_button: | T3  | :black_square_button: |
| -------- | -- | --------------------- | --- | --------------------- | --- | --------------------- |
| A        | 50 | :white_check_mark:    | 51  | :white_check_mark:    | 52  | :white_check_mark:    |
| B        | 37 | :warning:             | 48  | :warning:             | 52  | :white_check_mark:    |

In this scenario, provider B was significantly behind provider A, but was making forward progress, so was considered to be in warning status. Provider B slowly caught up to match provider A, and becomes healthy upon catching up.

## Scenario 5

| Provider | T1 | :black_square_button: | T2 | :black_square_button: | T3 | :black_square_button: |
| -------- | -- | --------------------- | -- | --------------------- | -- | --------------------- |
| A        | 50 | :white_check_mark:    | 51 | :white_check_mark:    | 52 | :white_check_mark:    |
| B        | 52 | :white_check_mark:    | 37 | :x:                   | 52 | :x:                   |

In this scenario, provider B reports data that lags behind its own previously reported block number. It is considered unhealthy for a few checks until it is consistently reporting increasing block numbers again.


## Scenario 6

| Provider | T1 | :black_square_button: | T2 | :black_square_button: | T3 | :black_square_button: | T4 | :black_square_button: |
| -------- | -- | --------------------- | -- | --------------------- | -- | --------------------- | -- | --------------------- |
| A        | 52 | :x:                   | 52 | :x:                   | 52 | :x:                   | 52 | :x:                   |
| B        | 77 | :white_check_mark:    | 78 | :white_check_mark:    | 60 | :x:                   | 61 | :warning:             |

In this scenario, provider A has been consistently behind, and thus is unhealthy. Provider B was reporting current blocks, but at Time T3 started reporting a block number older than had previously reported.

Provider A lags significantly behind the highest known block for the network, and is not making progress, thus is considered unhealthy.

In the short term, provider B will be considered unhealthy immediately upon going backwards, and once it starts moving forward again will shift to warning status.

Longer term, we plan to start tracking block hashes that would allow us to identify chain reorgs, that would allow us to determine if a move backwards is a reorg.