## Patroneos

Patroneos provides a layer of protection for EOSIO nodes designed to protect against some of the basic Denial of Service attack vectors.
It operates in two modes, filter and log. When running in filter mode, it proxies requests to the node through a series of middleware that perform various checks and logs success or failures to the log agents. When running in log mode, it receives logs from the filter agent and writes them to a file in a format that fail2ban can process.

### Configuration
The configuration can be update through the `config.json` file or by sending a `POST` request with the new body to `/config`.

### Middleware Verification Layer

* validateJSON
    * This middleware checks that the body provided can be parsed into a JSON object.

* validateSignatures
    * This middleware checks that the number of signatures on the transaction are not greater than the defined maximum.

* validateContract
    * This middleware checks that the contract is not in a list of blacklisted contracts.

* validateTransactionSize
    * This middleware checks that the size of the transaction data does not exceed the defined maximum.

