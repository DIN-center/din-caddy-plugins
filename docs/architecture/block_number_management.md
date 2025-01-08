# Dealing with Block Numbers Across Providers

There are several different ways in which a collection of providers might be considered healthy or unhealthy based on the blocknumbers of the various providers serving the same network. This document explores several of these scenarios.

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
| B        | 37 | :x:                   | 48  | :x:                   | 52 | :white_check_mark:    |

In this scenario, provider B was significantly behind provider A, and thus considered unhealthy. Provider B slowly caught up to match provider A, and becomes healthy upon catching up.

## Scenario 5

| Provider | T1 | :black_square_button: | T2 | :black_square_button: | T3 | :black_square_button: |
| -------- | -- | --------------------- | -- | --------------------- | -- | --------------------- |
| A        | 50 | :white_check_mark:    | 51 | :white_check_mark:    | 52 | :white_check_mark:    |
| B        | 52 | :white_check_mark:    | 37 | :x:                   | 52 | :x:                   |

In this scenario, provider B reports data that lags behind its own previously reported block number. It is considered unhealthy for a few checks until it is consistently reporting increasing block numbers again.


## Scenario 6

| Provider | T1 | :black_square_button: | T2 | :black_square_button: | T3 | :black_square_button: |
| -------- | -- | --------------------- | -- | --------------------- | -- | --------------------- |
| A        | 52 | :x:                   | 52 | :x:                   | 52 | :x:                   |
| B        | 77 | :white_check_mark:    | 78 | :white_check_mark:    | 60 | :question:            |

In this scenario, provider A has been consistently behind, and thus is unhealthy. Provider B was reporting current blocks, but at Time T3 started reporting a block number older than had previously reported.

It's unclear how we should treat Provider B after T3. Should we consider them healthy because they're the most ahead? Or should we treat them as unhealthy because we've previously seen a higher block number?

It's worth noting here that in some proof-of-work networks, it is actually possible for the block number to go down after a reorg as long as the totalDifficulty goes up.