#!/bin/bash

: ${MACHINE_WRAPPED:=true}

__machine_wrapper() {
    if [[ "$1" == use ]]; then
        # Special use wrapper
        shift 1
        case "$1" in
            -h|--help|"")
                cat <<EOF
Usage: machine use [OPTIONS] [arg...]

Set up environment for the Docker client

Description:
   Argument is either a machine name, clear, or swarm.

EOF
                ;;
            *)
                eval "$(machine env "$@")"
                ;;
        esac
    else
        command machine "$@"
    fi
}

if [[ ${MACHINE_WRAPPED} = true ]]; then
    alias machine=__machine_wrapper
fi
