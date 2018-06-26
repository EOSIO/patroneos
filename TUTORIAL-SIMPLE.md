## Tutorial for the basic Patroneos setup
### Requirements
1. A running instance of nodeos
2. A compiled binary of Patroneos

### Configuration
A sample configuration file is included in the repo and can be tweaked according to your individual needs.
```
listenIP   -- the ip address that Patroneos listens on (defaults to all ip addresses)
listenPort -- the port that Patroneos listens on

nodeosProtocol -- the protocol nodeos listens on (HTTP vs HTTPS)
nodeosUrl      -- the url nodeos is hosted at. This can be localhost if running Patroneos on the same machine as nodeos
nodeosPort     -- the port nodeos listens on. (defaults to 8888)

contractBlackList  -- an object that defines which contracts to blacklist. Should use the format contractName: true
maxSignatures      -- an integer that defines the maximum number of signatures a transaction can have
maxTransactionSize -- an integer in bytes that defines the maximum size of a transaction payload

logEndpoints    -- this configuration value is not needed for simple mode and can be set to an empty array
filterEndpoints -- this configuration value is not needed for simple mode and can be set to an empty array

logFileLocation -- this configuration value is not needed for simple mode and can be set to an empty string
```

### Infrastructure Setup
The simplest deployment of Patroneos is to run it on the same machine that nodeos is running on.

Example Structure:
```
--------------------------------------------------------
|    User's PC   | Request Data |     Nodeos Server    |
-------------------------------------------------------
|                |              |                      |
| User's machine |      -->     | Patroneos --> Nodeos |
|                |              |                      |
--------------------------------------------------------
```

### Example
Once Patroneos is configured correctly, all requests that would be made to nodeos should be made to patroneos instead.
Example Requests:
```
curl http://patroneos/v1/chain/get_info

{
    "server_version": "367ada2d",
	"head_block_num": 166,
	"last_irreversible_block_num": 165,
	"head_block_id": "000000a672abe2bdc8beb1a1e773d7d4ca5a5162b1c9af0f46f09ea8f9fe3b3f",
	"head_block_time": "2018-05-22T20:56:09",
	"head_block_producer": "eosio"
}
```
```
curl http://patroneos/v1/chain/get_code -X POST -d '{c}'

{
	"message": "INVALID_JSON",
	"code": 400
}
```
