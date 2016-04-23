## Logging Control

### Overview

Logging in the `peer` application and in the _shim_ interface to
chaincodes is programmed using facilities provided by the
`github.com/op/go-logging` package. This package supports

- Logging control based on the severity of the message
- Logging control based on the software _module_ generating the message
- Different pretty-printing options based on the severity of the message

All logs are currently directed to **stderr**, and the pretty-printing is
currently fixed. However global and module-level control of logging by
severity is provided for both users and developers.  There are currently no
formalized rules for the types of information provided at each severity level,
however when submitting bug reports the developers may want to see full logs
down to the DEBUG level.

In pretty-printed logs the logging level is indicated both by color and by a
4-character code, e.g, "ERRO" for ERROR, "DEBU" for DEBUG, etc.  In the
logging context a _module_ is an arbitrary name (string) given by developers
to groups of related messages.  In the pretty-printed example below, the
logging modules "peer", "rest" and "main" are generating logs.

    16:47:09.634 [peer] GetLocalAddress -> INFO 033 Auto detected peer address: 9.3.158.178:30303
    16:47:09.635 [rest] StartOpenchainRESTServer -> INFO 035 Initializing the REST service...
    16:47:09.635 [main] serve -> INFO 036 Starting peer with id=name:"vp1" , network id=dev, address=9.3.158.178:30303, discovery.rootnode=, validator=true

An arbitrary number of logging modules can be created at runtime, therefore
there is no "master list" of modules, and logging control constructs can not
check whether logging modules actually do or will exist.

### PEER

The logging level of the `peer` command can be controlled from the command
line for each invocation using the `--logging-level` flag, for example

    peer peer --logging-level=debug
	
The default logging level for each individual `peer` subcommand can also
be set in the **openchain.yaml** file. For example the key `logging.peer` sets
the default level for the `peer` subcommmand.

Logging severity levels are specified using case-insensitive strings chosen
from

    CRITICAL | ERROR | WARNING | NOTICE | INFO | DEBUG

The full logging level specification for the `peer` is of the form

    [<module>[,<module>...]=]<level>[:[<module>[,<module>...]=]<level>...]

A logging level by itself is taken as the overall default. Otherwise,
overrides for individual or groups of modules can be specified using the 

    <module>[,<module>...]=<level> 

syntax. Examples of <level> specifications (valid for both --logging-level and
**openchain.yaml** settings):

    info                                       - Set default to INFO
    warning:main,db=debug:chaincode=info       - Default WARNING; Override for main,db,chaincode
    chaincode=info:main=debug:db=debug:warning - Same as above



### GO chaincodes

There is currently no generic way to control logging in GO chaincodes. The
chaincode _shim_ currently defaults to DEBUG level logging, creating large
log files in the chaincode containers. The following is a suggested
workaround: First, add

    "github.com/op/go-logging"
	
to the _imports_ of the chaincode. Also add a line like

    logging.SetLevel(logging.WARNING, "")
	
as the first line of the chaincode `main` routine to suppress all but WARNING
and more severe errors.

