# fabric.tcl - A Tcl support package for Hyperledger fabric scripts

# Copyright IBM Corp. 2016. All Rights Reserved.
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
# 		 http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Note: The logging level is controlled by the 'fabric' module.

package require http
package require json
package require utils
package require yaml

package provide fabric 0.0
namespace eval ::fabric {}

############################################################################
# devops i_peer i_method i_query {i_retry 0}

# Make a REST API 'devops' query. The i_peer is the full host:port
# address. The i_method must be 'deploy', 'invoke' or 'query'.

# If 'i_retry' is greater than 0, then HTTP failures are logged but retried up
# to that many times. Incorrectly formatted data returned from a valid query
# is never retried. We currently do not implememnt retry backoffs - it's
# pedal-to-the-metal. 

proc ::fabric::devops {i_peer i_method i_query {i_retry 0}} {

    for {set retry $i_retry} {$retry >= 0} {incr retry -1} {
        
        if {[catch {
            ::http::geturl http://$i_peer/devops/$i_method -query $i_query
        } token]} {
            if {$retry > 0} {
                if {$retry == $i_retry} {
                    warn fabric \
                        "fabric::devops/$i_method $i_peer : " \
                        "Retrying after catastrophic HTTP error"
                }
                continue
            }
            if {($retry == 0) && ($i_retry != 0)} {
                err fabric \
                    "fabric::devops/$i_method $i_peer : " \
                    "Retry limit ($i_retry) hit after " \
                    "catastrophic HTTP error : Aborting"
            }
            errorExit \
                "fabric::devops/$i_method $i_peer : ::http::geturl failed\n" \
                $::errorInfo
        }
        
        if {[http::ncode $token] != 200} {
            if {$retry > 0} {
                if {$retry == $i_retry} {
                    warn fabric \
                        "fabric::devops/$i_method $i_peer : " \
                        "Retrying after HTTP error return"
                }
                if {($retry == 0) && ($i_retry != 0)} {
                    err fabric \
                        "fabric::devops/$i_method $i_peer : " \
                        "Retry limit ($i_retry) hit after " \
                        "HTTP error return : Aborting"
                }
                continue
            }
            err err \
                "FABRIC '$i_method' transaction to $i_peer failed " \
                "with ncode = '[http::ncode $token]'; Aborting\n"
            err err "Full dump of HTTP response below:"
            foreach {k v} [array get $token] {err err "    $k $v"}
            exit 1
        }

        set response [http::data $token]
    
        set err [catch {
            set parse [json::json2dict $response]
            set ok [dict get $parse OK]
            switch $i_method {
                deploy -
            invoke {
                set result [dict get $parse message]
            }
            query {
                set result $ok
            }
            default {
                error "Unrecognized method $i_method"
            }
        }
        }]
        http::cleanup $token
    
        if {$err} {
            err err \
                "FABRIC '$i_method' response from $i_peer " \
                "is malformed/unexpected"
            err err "Chaincode : $i_chaincode"
            err err "Full response below:"
            err err $response
            exit 1
        }
        if {$retry != $i_retry} {
            note fabric \
                "fabric::devops/$i_method $i_peer : " \
                "Success after [expr {$i_retry - $retry}] HTTP retries"
        }
        break
    }

    return $result
}
    

############################################################################
# deploy i_peer i_user i_chaincode i_fn i_args {i_retry 0}

# Deploy a GOLANG chaincode to the network. The i_peer is the full network
# address (host:port) of the REST API port of the peer.  If i_user is
# non-empty then this will be a secure transaction. The constructor will apply
# i_fn to i_args. Note that i_args is a normal Tcl list. This routine will
# convert i_args into a JSON array, wrapping each element of i_args in double
# quotes. i_fn will also be properly quoted.

# See ::fabric::devops{} for a discussion of the 'i_retry' parameter.

proc ::fabric::deploy {i_peer i_user i_chaincode i_fn i_args {i_retry 0}} {

    set template {
        {
            "type" : "GOLANG",
            "chaincodeID" : {
                "path" : "$i_chaincode"
            },
            "ctorMsg" : {
                "function" : "$i_fn",
                "args" : [$args]
            },
            "secureContext": "$i_user"
        }
    }

    set args [argify $i_args]
    set query [subst -nocommand $template]

    return [devops $i_peer deploy $query $i_retry]

}

############################################################################
# devModeDeploy i_peer i_user i_chaincode i_fn i_args {i_retry 0}

# Issue a "deploy" transaction for a pre-started chaincode in development
# mode. Here, the i_chaincode is a user-specified name. All of the other
# arguments are otherwise the same as for deploy{}.

proc ::fabric::devModeDeploy {i_peer i_user i_chaincode i_fn i_args {i_retry 0}} {

    set template {
        {
            "type" : "GOLANG",
            "chaincodeID" : {
                "name" : "$i_chaincode"
            },
            "ctorMsg" : {
                "function" : "$i_fn",
                "args" : [$args]
            },
            "secureContext": "$i_user"
        }
    }

    set args [argify $i_args]
    set query [subst -nocommand $template]

    return [devops $i_peer deploy $query $i_retry]

}

############################################################################
# invoke i_peer i_user i_chaincode i_fn i_args {i_retry 0}

# Invoke a GOLANG chaincode on the network. The i_peer is the full network
# address (host:port) of the REST API port of the peer. If i_user is non-empty
# then this will be a secure transaction. The i_chaincodeName is the hash used
# to identify the chaincode. The invocation will apply i_fn to i_args. Note
# that i_args is a normal Tcl list. This routine will convert i_args into a
# JSON array, wrapping each element of i_args in double quotes. i_fn will also
# be properly quoted.

# See ::fabric::devops{} for a discussion of the 'i_retry' parameter.

proc ::fabric::invoke {i_peer i_user i_chaincodeName i_fn i_args {i_retry 0}} {

    set template {
        {
            "chaincodeSpec" : {"type" : "GOLANG",
                "chaincodeID" : {
                    "name" : "$i_chaincodeName"
                },
                "ctorMsg" : {
                    "function" : "$i_fn",
                    "args" : [$args]
                },
                "secureContext": "$i_user"
            }
        }
    }

    set args [argify $i_args]
    set query [subst -nocommand $template]

    return [devops $i_peer invoke $query $i_retry]
}


############################################################################
# query i_peer i_user i_chaincodeName i_fn i_args {i_retry 0}

# Query a GOLANG chaincode on the network. The i_peer is the full network
# address (host:port) of the REST API port of the peer. If i_user is non-empty
# then this will be a secure transaction. The i_chaincodeName is the hash used
# to identify the chaincode. The query will apply i_fn to i_args. Note that
# i_args is a normal Tcl list. This routine will convert i_args into a JSON
# array, wrapping each element of i_args in double quotes. i_fn will also be
# properly quoted. The query result (currently assumed to be a string) is
# returned.

# See ::fabric::devops{} for a discussion of the 'i_retry' parameter.

proc ::fabric::query {i_peer i_user i_chaincodeName i_fn i_args {i_retry 0}} {

    set template {
        {
            "chaincodeSpec" : {"type" : "GOLANG",
                "chaincodeID" : {
                    "name" : "$i_chaincodeName"
                },
                "ctorMsg" : {
                    "function" : "$i_fn",
                    "args" : [$args]
                },
                "secureContext": "$i_user"
            }
        }
    }

    set args [argify $i_args]
    set query [subst -nocommand $template]

    return [devops $i_peer query $query $i_retry]
}


############################################################################
# checkForLocalDockerChaincodes i_nPeers i_chaincodeNames

# Given a system with i_nPeers peers, e.g., a Docker compose setup, return 1 if
# we see i_nPeers copies of each chaincode name in the Docker 'ps' listing of
# running containers, else 0.

proc ::fabric::checkForLocalDockerChaincodes {i_nPeers i_chaincodeNames} {

    foreach name $i_chaincodeNames {

        catch [list exec docker ps -f status=running | grep -c $name] count
        if {$count != $i_nPeers} {return 0}
    }
    return 1
}


############################################################################
# dockerLocalPeerContainers
# dockerLocalPeerIPs

# Return a list of docker container IDs for all running containers based on
# the 'hyperledger-peer' image.

proc ::fabric::dockerLocalPeerContainers {} {

    # This does not seem to work; membersrvc may (appear to?) be built from
    # the hyperledger-peer image.
    #return [exec docker ps -q -f status=running -f ancestor=hyperledger-peer]
    
    return [exec docker ps --format="{{.Image}}\ {{.ID}}" | \
                grep ^hyperledger-peer | cut -f 2 -d " " ]
}


# Return a list of IP addresses of running containers based on the
# 'hyperledger-peer' image. The IP addresses are returned in sorted order.

proc ::fabric::dockerLocalPeerIPs {} {

    return [lsort \
                [mapeach container [dockerLocalPeerContainers] { 
                    exec docker inspect \
                        --format {{{.NetworkSettings.IPAddress}}} $container
                }]]
}

# Return a list of pairs of {<name> <IP>} for running containers based on the
# 'hyperledger-peer' image. The pairs are returned sorted on the container
# name.

proc ::fabric::dockerLocalPeerNamesAndIPs {} {

    set l \
        [mapeach container [dockerLocalPeerContainers] {
            list \
                 [exec docker inspect --format {{{.Name}}} $container] \
                 [exec docker inspect \
                      --format {{{.NetworkSettings.IPAddress}}} $container]
        }]
    return [lsort -dictionary -index 0 $l]
}


############################################################################
# Security
############################################################################

############################################################################
# caLogin user secret

# Log in a user

proc ::fabric::caLogin {i_peer i_user i_secret} {

    set template {
        {
            "enrollId"     : "$i_user",
            "enrollSecret" : "$i_secret"
        }
    }

    set query [subst $template]

    set token [::http::geturl http://$i_peer/registrar -query $query]
    if {[http::ncode $token] != 200} {
        errorExit "fabric::caLogin : Login failed for user $i_user on peer $i_peer"
    }
    http::cleanup $token
}

    
############################################################################
# argify i_args

# Convert a Tcl list to a list of quoted arguments with commas to satisfy the
# JSON format. This needs to be done as a string (rather than as a list),
# otherwise it will be {} quoted when substituted.

proc ::fabric::argify {i_args} {

    set args ""
    set comma ""
    foreach arg $i_args {
        set args "$args$comma\"$arg\""
        set comma ,
    }
    return $args
}
