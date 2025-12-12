# DIN Marketplace Contract

## Overview

The DIN Marketplace Contract is deployed on Linea and manages all subscription agreements between consumers and providers. It handles plan registration, agreement creation, payment escrow, and usage tracking.

## Contract Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         DINMarketplace.sol                                   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                        Plan Management                               │   │
│   │   - registerPlan()                                                   │   │
│   │   - updatePlan()                                                     │   │
│   │   - deactivatePlan()                                                 │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                     Agreement Management                             │   │
│   │   - createAgreement()                                                │   │
│   │   - renewAgreement()                                                 │   │
│   │   - cancelAgreement()                                                │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                        Usage Tracking                                │   │
│   │   - recordUsage() (for request count / credits)                      │   │
│   │   - batchRecordUsage()                                               │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                        Fund Management                               │   │
│   │   - withdrawFunds() (provider)                                       │   │
│   │   - refundConsumer() (on cancellation)                               │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Data Structures

### Enums

```solidity
enum PlanType {
    Unlimited,      // Fixed monthly, unlimited requests
    RequestCount,   // Pay per X requests
    CreditBased     // Pay per credit (method-specific)
}

enum AgreementStatus {
    Active,
    Expired,
    Cancelled,
    Exhausted       // For usage-based: ran out of requests/credits
}
```

### Plan Structures

```solidity
struct Plan {
    uint256 id;
    address provider;
    string networkName;
    PlanType planType;
    uint256 price;              // Interpretation depends on planType
    uint256 units;              // For RequestCount: requests per purchase
                                // For CreditBased: credits per purchase
                                // For Unlimited: not used
    uint256 validityDays;
    bool active;
    uint256 createdAt;
}

// For credit-based plans, method costs stored separately
struct MethodCost {
    uint256 planId;
    string method;
    uint256 credits;
}
```

### Agreement Structure

```solidity
struct Agreement {
    uint256 id;
    uint256 planId;
    address consumer;
    address provider;
    uint256 startTime;
    uint256 endTime;
    uint256 paidAmount;
    uint256 totalUnits;         // Total requests or credits purchased
    uint256 usedUnits;          // Used requests or credits
    AgreementStatus status;
}
```

## Core Functions

### Plan Management

```solidity
/// @notice Register a new pricing plan
/// @param networkName The network this plan covers (e.g., "ethereum-mainnet")
/// @param planType The type of pricing model
/// @param price For Unlimited: monthly price in USDC
///              For RequestCount: price per million requests
///              For CreditBased: price per credit
/// @param units For RequestCount: requests per purchase unit
///              For CreditBased: credits per purchase unit
/// @param validityDays How long purchased units remain valid
function registerPlan(
    string memory networkName,
    PlanType planType,
    uint256 price,
    uint256 units,
    uint256 validityDays
) external returns (uint256 planId) {
    require(price > 0, "Price must be positive");
    require(validityDays > 0, "Validity must be positive");

    planId = nextPlanId++;
    plans[planId] = Plan({
        id: planId,
        provider: msg.sender,
        networkName: networkName,
        planType: planType,
        price: price,
        units: units,
        validityDays: validityDays,
        active: true,
        createdAt: block.timestamp
    });

    providerPlans[msg.sender].push(planId);
    networkPlans[networkName].push(planId);

    emit PlanRegistered(planId, msg.sender, networkName, planType);
}

/// @notice Set method costs for a credit-based plan
function setMethodCosts(
    uint256 planId,
    string[] memory methods,
    uint256[] memory credits
) external {
    require(plans[planId].provider == msg.sender, "Not plan owner");
    require(plans[planId].planType == PlanType.CreditBased, "Not credit plan");
    require(methods.length == credits.length, "Length mismatch");

    for (uint i = 0; i < methods.length; i++) {
        methodCosts[planId][methods[i]] = credits[i];
    }

    emit MethodCostsUpdated(planId, methods, credits);
}
```

### Agreement Management

```solidity
/// @notice Create a new agreement (subscribe to a plan)
/// @param planId The plan to subscribe to
/// @param quantity For Unlimited: number of months
///                 For RequestCount: number of request units
///                 For CreditBased: number of credit units
function createAgreement(
    uint256 planId,
    uint256 quantity
) external returns (uint256 agreementId) {
    Plan storage plan = plans[planId];
    require(plan.active, "Plan not active");
    require(quantity > 0, "Quantity must be positive");

    // Calculate payment amount
    uint256 paymentAmount = plan.price * quantity;

    // Calculate total units and end time based on plan type
    uint256 totalUnits;
    uint256 endTime;

    if (plan.planType == PlanType.Unlimited) {
        totalUnits = 0;  // Unlimited doesn't track
        endTime = block.timestamp + (quantity * 30 days);
    } else {
        totalUnits = plan.units * quantity;
        endTime = block.timestamp + (plan.validityDays * 1 days);
    }

    // Transfer payment from consumer
    require(
        paymentToken.transferFrom(msg.sender, address(this), paymentAmount),
        "Payment failed"
    );

    // Create agreement
    agreementId = nextAgreementId++;
    agreements[agreementId] = Agreement({
        id: agreementId,
        planId: planId,
        consumer: msg.sender,
        provider: plan.provider,
        startTime: block.timestamp,
        endTime: endTime,
        paidAmount: paymentAmount,
        totalUnits: totalUnits,
        usedUnits: 0,
        status: AgreementStatus.Active
    });

    // Track in mappings
    consumerAgreements[msg.sender].push(agreementId);
    providerAgreements[plan.provider].push(agreementId);

    // Index by consumer+provider+network for fast lookups
    bytes32 key = keccak256(abi.encodePacked(msg.sender, plan.provider, plan.networkName));
    activeAgreementsByKey[key] = agreementId;

    emit AgreementCreated(agreementId, planId, msg.sender, plan.provider);
}

/// @notice Get active agreement for consumer-provider-network triple
function getActiveAgreement(
    address consumer,
    address provider,
    string memory networkName
) external view returns (Agreement memory) {
    bytes32 key = keccak256(abi.encodePacked(consumer, provider, networkName));
    uint256 agreementId = activeAgreementsByKey[key];

    if (agreementId == 0) {
        revert("No active agreement");
    }

    Agreement storage agreement = agreements[agreementId];

    // Check if still valid
    if (agreement.status != AgreementStatus.Active) {
        revert("Agreement not active");
    }
    if (agreement.endTime < block.timestamp) {
        revert("Agreement expired");
    }

    return agreement;
}
```

### Usage Tracking

```solidity
/// @notice Record usage for a usage-based agreement
/// @dev Only callable by the provider
function recordUsage(
    uint256 agreementId,
    uint256 unitsUsed
) external {
    Agreement storage agreement = agreements[agreementId];
    require(agreement.provider == msg.sender, "Not provider");
    require(agreement.status == AgreementStatus.Active, "Not active");

    Plan storage plan = plans[agreement.planId];
    require(plan.planType != PlanType.Unlimited, "Unlimited has no usage");

    agreement.usedUnits += unitsUsed;

    // Check if exhausted
    if (agreement.usedUnits >= agreement.totalUnits) {
        agreement.status = AgreementStatus.Exhausted;
        emit AgreementExhausted(agreementId);
    }

    emit UsageRecorded(agreementId, unitsUsed, agreement.usedUnits);
}

/// @notice Batch record usage for multiple agreements
function batchRecordUsage(
    uint256[] calldata agreementIds,
    uint256[] calldata unitsUsed
) external {
    require(agreementIds.length == unitsUsed.length, "Length mismatch");

    for (uint i = 0; i < agreementIds.length; i++) {
        Agreement storage agreement = agreements[agreementIds[i]];
        if (agreement.provider == msg.sender &&
            agreement.status == AgreementStatus.Active) {
            agreement.usedUnits += unitsUsed[i];

            if (agreement.usedUnits >= agreement.totalUnits) {
                agreement.status = AgreementStatus.Exhausted;
            }
        }
    }

    emit BatchUsageRecorded(msg.sender, agreementIds.length);
}
```

### Fund Management

```solidity
/// @notice Provider withdraws earned funds
function withdrawFunds(uint256 amount) external {
    require(providerBalance[msg.sender] >= amount, "Insufficient balance");

    providerBalance[msg.sender] -= amount;
    require(paymentToken.transfer(msg.sender, amount), "Transfer failed");

    emit FundsWithdrawn(msg.sender, amount);
}

/// @notice Process agreement - release funds to provider
/// @dev Called when agreement expires or is completed
function settleAgreement(uint256 agreementId) external {
    Agreement storage agreement = agreements[agreementId];
    require(
        agreement.endTime < block.timestamp ||
        agreement.status == AgreementStatus.Exhausted,
        "Agreement still active"
    );
    require(agreement.paidAmount > 0, "Already settled");

    // For simplicity, entire amount goes to provider
    // (Pro-rata refunds could be added for cancellations)
    providerBalance[agreement.provider] += agreement.paidAmount;
    agreement.paidAmount = 0;

    if (agreement.status == AgreementStatus.Active) {
        agreement.status = AgreementStatus.Expired;
    }

    // Clear active agreement index
    Plan storage plan = plans[agreement.planId];
    bytes32 key = keccak256(abi.encodePacked(
        agreement.consumer,
        agreement.provider,
        plan.networkName
    ));
    delete activeAgreementsByKey[key];

    emit AgreementSettled(agreementId, agreement.provider);
}
```

## Events

```solidity
event PlanRegistered(
    uint256 indexed planId,
    address indexed provider,
    string networkName,
    PlanType planType
);

event MethodCostsUpdated(
    uint256 indexed planId,
    string[] methods,
    uint256[] credits
);

event AgreementCreated(
    uint256 indexed agreementId,
    uint256 indexed planId,
    address indexed consumer,
    address provider
);

event AgreementExhausted(uint256 indexed agreementId);

event AgreementSettled(
    uint256 indexed agreementId,
    address indexed provider
);

event UsageRecorded(
    uint256 indexed agreementId,
    uint256 unitsUsed,
    uint256 totalUsed
);

event BatchUsageRecorded(
    address indexed provider,
    uint256 count
);

event FundsWithdrawn(
    address indexed provider,
    uint256 amount
);
```

## View Functions

```solidity
/// @notice Get all plans for a provider
function getProviderPlans(address provider)
    external view returns (uint256[] memory);

/// @notice Get all plans for a network
function getNetworkPlans(string memory networkName)
    external view returns (uint256[] memory);

/// @notice Get all active agreements for a consumer
function getConsumerAgreements(address consumer)
    external view returns (uint256[] memory);

/// @notice Get remaining units for an agreement
function getRemainingUnits(uint256 agreementId)
    external view returns (uint256) {
    Agreement storage agreement = agreements[agreementId];
    if (agreement.totalUnits == 0) {
        return type(uint256).max;  // Unlimited
    }
    return agreement.totalUnits - agreement.usedUnits;
}

/// @notice Check if agreement is valid (active, not expired, has units)
function isAgreementValid(uint256 agreementId)
    external view returns (bool) {
    Agreement storage agreement = agreements[agreementId];
    return agreement.status == AgreementStatus.Active &&
           agreement.endTime > block.timestamp &&
           (agreement.totalUnits == 0 ||
            agreement.usedUnits < agreement.totalUnits);
}
```

## Storage Layout

```solidity
contract DINMarketplace {
    // Payment token (USDC on Linea)
    IERC20 public immutable paymentToken;

    // Counters
    uint256 public nextPlanId = 1;
    uint256 public nextAgreementId = 1;

    // Plan storage
    mapping(uint256 => Plan) public plans;
    mapping(address => uint256[]) public providerPlans;
    mapping(string => uint256[]) public networkPlans;

    // Method costs for credit-based plans
    mapping(uint256 => mapping(string => uint256)) public methodCosts;
    mapping(uint256 => uint256) public defaultMethodCost;

    // Agreement storage
    mapping(uint256 => Agreement) public agreements;
    mapping(address => uint256[]) public consumerAgreements;
    mapping(address => uint256[]) public providerAgreements;

    // Fast lookup: consumer+provider+network -> agreementId
    mapping(bytes32 => uint256) public activeAgreementsByKey;

    // Provider balances (earned from agreements)
    mapping(address => uint256) public providerBalance;
}
```

## Gas Optimization Notes

1. **Batch usage updates** - Providers should batch usage updates to amortize gas
2. **Active agreement index** - O(1) lookup for verifying agreements
3. **No enumeration** - Arrays only for off-chain queries; on-chain uses mappings
4. **Minimal storage writes** - Usage recorded but not synced every request

## Security Considerations

1. **Reentrancy** - Use checks-effects-interactions pattern
2. **Overflow** - Solidity 0.8+ has built-in overflow checks
3. **Access control** - Provider-only functions for their own plans/agreements
4. **Front-running** - Plan prices locked at creation time
5. **Griefing** - Minimum payment amounts prevent dust attacks
