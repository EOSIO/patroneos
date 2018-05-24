# Patroneos

Patroneos provides a layer of protection for EOSIO nodes designed to protect against some of the basic Denial of Service attack vectors. It runs in a simple configuration and a more advanced configuration.

## Simple Configuration
The simple configuration is designed to simply drop requests that are invalid or could cause unnecessary load on the node. This is done by running the request through a set of middleware (described below) that apply rules to the request. If a request passes all the middleware, it is forwarded to the node with the response returned to the user. Otherwise, an error code and the failure condition is returned to the user.

```
Successful request data flow
-----------------------------
User --> Patroneos --> Nodeos --> Patroneos --> User
```
```
Failed request data flow
-------------------------
User --> Patroneos --> User
```

To setup Patroneos in the simple configuration, a user just needs nodeos running, a compiled patroneos binary, and a correct `config.json`. See [Basic Patroneos Setup](TUTORIAL-SIMPLE.md) for a walkthrough of setting up and using Patroneos.

#### Middleware Verification Layer

* validateJSON
    * This middleware checks that the body provided can be parsed into a JSON object.

* validateMaxSignatures
    * This middleware checks that the number of signatures on the transaction are not greater than the defined maximum.

* validateContract
    * This middleware checks that the contract is not in a list of blacklisted contracts.

* validateTransactionSize
    * This middleware checks that the size of the transaction data does not exceed the defined maximum.

## Advanced Configuration
The advanced configuration works in coordination with fail2ban to ban users that repeatedly submit blocked requests. It requires a reverse proxy, patroneos running in fail2ban-relay mode, fail2ban, patroneos running in filter mode, and nodeos.

The advanced configuration is defined more in depth at [Advanced Patroneos Setup](TUTORIAL-ADVANCED.md)

## Data Flow Diagram

![Data Flow Diagram](patroneos-diagram.png "Patroneos Data Flow Diagram")
