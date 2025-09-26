### Watcher-Driven Dynamic Load Balancing

@author: @noleto

@status: in review

date: 2025-01-24
updated: 2025-09-26

## Introduction

The DIN Router currently routes traffic evenly among providers of the same priority level. "Evenly" means each eligible and healthy provider gets an equal share of the traffic. There's no consideration of provider performance or data quality in this distribution.
This document proposes a way to improve the routing algorithm by introducing a "Provider Watcher Score" that reflects how well each provider is performing in terms of data consistency and latency over time. This score will be used to dynamically weight traffic distribution, hence the name "dynamic load balacing." Better-performing providers will receive a proportionally larger share of requests. The ultimate goal is to improve service quality for DIN developers.

## Computing Watcher Scores

Watcher Scores are a way to assess the reliability of a JSON RPC Provider from a high level perspective. Providers will be measured based on a set of criteria, forming the "Provider Watcher Score." This score is calculated using data provided by Watchers, which perform regular checks and latency measurement. The criteria currently monitored by Watchers include:

- Block Number Consistency (BNC): This metric ensures that the block number either monotonically increases or remains consistent.
- Consistency of Non-State Data for a Block (CNSB): This metric verifies that non-state data for a given block hash remains consistent over time.
- Request Latency (RLAT): This metric measures the elapsed time (in milliseconds) between sending a request and receiving a response, targeting the provider endpoint directly (without the DIN Router).

These metrics are evaluated per network and per provider. The formula to compute the Provider Watcher Score for a provider (e.g., P1) in a given network is:

$$Φ({P1}) = BNC(P1) \times W_{bnc} + CNSB({P1}) \times W_{cnsb} + RLAT({P1} | {P(95)}) \times W_{rlat}$$

Where:
* Φ (Phi) is the Watcher Score (range: [0,1])
* $W_{bnc}$, $W_{cnsb}$, $W_{rlat}$ are the weights assigned to each metric (weights sum to 1)
* BNC, CNSB, and RLAT are scaled metrics (range: [0,1]) for Block Number Consistency, Consistency of Non-State Data, and Request Latency, respectively and are provided by the Watcher

## Technical Details for the Watcher Score

The current proposal defines a set of abstractions and implementations to calculate the Watcher Score for a provider.

### Abstractions
- `ProviderMetric`: A provider metric is a number between 0 and 1 that can be used to measure the quality of a provider for a given criteria. For example, the block number consistency metric ensures the consistency rate of a provider's block number.
- `Score`: A score is a number in the range [0,1] that represents how well a provider is performing globally.
- `ProviderMetricGenerator`: A provider metric generator is responsible for generating the same metric type for all providers in a given network. In mathematical terms, a provider metric generator can be represented as a function: 

    $$\mathrm{PMG}(Network) \rightarrow [x_{p_1}, x_{p_2}, ..., x_{p_n}]$$ 
    where $x_{p_i}$ is a provider metric of type $x$ for provider $p_i$ and $Network$ is the network to which provider $p_i$ belongs.
- `ProviderMetricCombiner`: A provider metric combiner is responsible for combining (reducing) different types of metrics for a provider in a given network into a single score. In mathematical terms, a provider metric combiner can be represented as a function: 
    $$\mathrm{PMC}(x_{p_1}, y_{p_1}, ..., z_{p_1}) \rightarrow S_{p_1}$$ 
    where $S_{p_1}$ is a score for provider $p_1$ and $x_{p_1}, y_{p_1}, ..., z_{p_1}$ are the different metrics in the range [0,1] measured by the Watcher for provider $p_1$ in a given network.
- `ScoreTransformer`: A score transformer is responsible for transforming a set of scores in a input domain $K$ into another set of scores in a output domain $L$ by applying a mathematical function $\xi$. In mathematical terms, a score transformer can be represented as a function:
    $$\mathrm{ST}(S_1, S_2, ..., S_n) = \xi(S_1, S_2, ..., S_n) \rightarrow S_1', S_2', ..., S_n'$$
    where $S_1', S_2', ..., S_n'$ are the transformed scores in the output domain $L$ and $S_1, S_2, ..., S_n$ are the original scores in the input domain $K$. Note that domains $K$ and $L$ are both in the range [0,1].
- `ScoreFormula`: A score formula defines which components will be used to compute the score (a metric) and how to combine them to produce a score for a provider on a given network.

### Implementation

At the core of the watcher score algorithm is the `WatcherScoreManager` which is responsible for computing and storing the score for a provider on a given network. The `WatcherScoreManager` orchestrates the different components to compute the score but does not compute the score itself. It delegates the computation to the `ScoreFormula`s where all the logic is implemented.
The UML diagram below shows the relationship between the different components:

<details><summary>UML Source for Watcher Score Manager and its components</summary>

```plantuml
@startuml
skinparam style strictuml
skinparam classAttributeIconSize 0

interface ProviderMetric {
+MetricID(): string
+ProviderID(): string
+Value(): float64
+LastUpdated(): Time
}
class Score {
+HasValue(): bool
+Value(): float64
+LastUpdated(): Time
}
interface ProviderMetricGenerator {
+GenerateMetrics(network: string): List<ProviderMetric>
}
interface ProviderMetricCombiner {
+CombineMetrics(metrics: List<ProviderMetric>): Score
}
interface ScoreTransformer {
+TransformScore(scores: Map<String,Score>): Map<String,Score>
}
class ScoreFormula {
+Network: String
+MetricGenerators: List<ProviderMetricGenerator>
+MetricCombiner: ProviderMetricCombiner
+ScoreTransformer: ScoreTransformer
}
class WatcherScoreManager {
+ComputeScores(provider: Provider, network: Network)
+GetScore(network: string, providerID: string): Score
+GetAllScores(network string): Map<string, Score>
+StartPeriodicUpdates(frequency Duration)
}
WatcherScoreManager o-- ScoreFormula
ScoreFormula o-- ProviderMetricGenerator
ScoreFormula o-- ProviderMetricCombiner
ScoreFormula o-- ScoreTransformer
@enduml

```
</details>

![classdiagram](class-diagram.png)

### Score Formula

The score formula is the abstraction that defines how a score for a provider on a given network is computed. The score formula is implemented as a `ScoreFormula` and is responsible for:
- Defining which metrics will be used to compute the score for a provider on a given network.
- Combining the metrics into a single score for a provider on a given network.
- Transforming the score into a final score for a provider on a given network.

Note that the score formula is defined per network. This means that each network will have its own score formula. The default implementation uses the same formula for all networks.

### Combiners and Transformers

As discussed in the [Abstractions](#abstractions) section, the `ProviderMetricCombiner` and `ScoreTransformer` are responsible for combining and transforming the metrics into a single score for a provider on a given network.
This encapsulation allows developers to implement their own combiners and transformers if they want to use a different mathematical expression to compute the score. By default, the system uses the combiner `WeightedCombiner` defined in the `combiners.go` file. The `WeightedCombiner` uses weights to aggregate the metrics into a single score such in the expression:

$$WC(M_1, M_2, ..., M_n) = M_1 \times W_{m_1} + M_2 \times W_{m_2} + ... + M_n \times W_{m_n}$$


There are 3 transformers implemented:
- `HighPassThroughTransformer`: This transformer passes through scores above a given cutoff value. This is useful to avoid having providers with very low scores to be included in the routing algorithm. This is the default transformer.
- `ShareOfTotalTransformer`: This transformer converts a set of score for different providers into a score that represents the percentage this provider contributes to the total score. This ensures that the sum of all providers' score for a given network is 1. This is useful to transform the score into a traffic weight distribution.
- `EWMATransformer`: This transformer applies an Exponential Weighted Moving Average (EWMA) function to the score as a way to smooth the score over time. This is useful to avoid sudden changes in the score that could be caused by a single metric [1].

### Watcher metrics

The watcher score algorithm is completely agnostic to the system that is used to collect the metrics. The only requirement is that the metrics are scaled to the range [0,1] and that they are available for each provider in a given network.

This proposal delivers a built-in score based on current Watcher metrics. The Watcher metrics are:
- `WatcherBlockNumberConsistency`: This is the implementation of the block number consistency metric.
- `WatcherConsistencyOfNonStateData`: This is the implementation of the consistency of non-state data metric.
- `WatcherRequestLatency`: This is the implementation of the request latency metric.

The Watcher metrics are implemented in the `watcher_metrics.go` file and require a Watcher client to fetch the raw data from the Watcher API. All the metrics are implemented as `ProviderMetricGenerator`s. This is the only link between the watcher score algorithm and the Watcher.

Note that Watchers don't provide "scores" but rather raw metrics. Developers are free to leverage the metrics provided by the Watcher to implement any derived score or monitoring system they want.

Also, developers can implement their own metrics by implementing the `ProviderMetricGenerator` interface and use any other system to collect the metrics (e.g. a custom monitoring system).

### Sequence diagram

The sequence diagram below shows the flow of the watcher score algorithm.

<details><summary>UML Sequence diagram for Watcher Score calculation</summary>

```plantuml
@startuml
skinparam style strictuml

participant "WatcherScoreManager" as WSM
participant "ScoreFormula" as SF
participant "MetricGenerator" as MG
participant "MetricCombiner" as MC
participant "ScoreTransformer" as ST

[-> WSM: ComputeScores()
activate WSM

loop for each formula
    WSM -> MG: GenerateMetrics(network)
    activate MG
    MG --> WSM: providerMetrics[]
    deactivate MG

    WSM -> WSM: Group metrics by provider

    loop for each provider
        WSM -> MC: CombineMetrics(metrics[])
        activate MC
        MC --> WSM: rawScore
        deactivate MC
    end

    WSM -> ST: TransformScore(rawScores)
    activate ST
    ST --> WSM: transformedScores
    deactivate ST

    WSM -> WSM: Update scores map
end

[<-- WSM: return
deactivate WSM

@enduml
```
</details>

![sequence](sequence-diagram.png)

## Integration with the DIN Router

The DIN Router integrates watcher scores through the `WatcherScoreManager` to enable dynamic traffic distribution among providers. This feature requires both the registry and dynamic load balacing to be enabled.

### Smart Routing

This proposal adds a new selection policy called `din_score_based_selector` that considers provider watcher scores when distributing traffic. This selector is integrated with the existing `din_select` selector and will be used when the `smart_routing` directive is enabled.

The selection process follows these rules:

1. Session affinity takes precedence (as previously defined in the `din_select` selector) - if a request specifies a session, it will always be routed to the same provider regardless of scores
2. For non-session requests (this is where the new `din_score_based_selector` comes into play):
   - With dynamic load balacing enabled: Traffic is distributed proportionally based on provider scores (scaled to the range 0-100). It uses Weighted Random algorithm [2] to distribute the traffic where provider scores are the relative odds in selection algorithm.
   - With dynamic load balacing disabled: Traffic is distributed evenly (randomly) among all providers.

### Configuration

Dynamic load balacing is configured within the `din_registry` directive:

```caddy
din_registry {
    smart_routing {
        enabled true                                # Enables score-based routing
        watcher_endpoint https://watcher.din.com    # Source of provider metrics
        watcher_api_key key                         # Authentication for watcher API
        sync_score_enabled true                     # Enables periodic score updates
        sync_score_interval_secs 300                # Score update frequency (5 minutes)
    }
}
```

The router synchronizes scores with the watcher service at regular intervals aligned with registry epochs. This ensures that routing decisions are based on recent performance data while maintaining system stability (score doesn't change over the same epoch).

### Corner Cases

The dynamic load balacing system handles several edge cases to ensure stable operation:

1. **New or Unmonitored Providers**: 
   - When a provider has no watcher score yet (e.g., newly registered provider or not yet monitored by Watcher)
   - The system assigns a default score of 50 to ensure the provider receives a fair share of traffic while building its watcher

2. **Stale or Missing Data**:
   - If Watcher data becomes stale or unavailable, the system:
     - Continues using last known scores during a grace period (default grace period is 60 minutes)
     - After the grace period, reverts to a default score of 50 for affected providers
   - This approach maintains system stability while gracefully degrading to fair distribution when needed


## References

[1] See [Exponential Smoothing](https://en.wikipedia.org/wiki/Exponential_smoothing#Basic_(simple)_exponential_smoothing) for more details.

[2] See [Weighted Random](https://dev.to/jacktt/understanding-the-weighted-random-algorithm-581p) for more details.