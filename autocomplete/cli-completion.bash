#!/bin/bash

: ${PROG:=$(basename ${BASH_SOURCE})}

_cli_bash_autocomplete() {
    local cur opts base
    cur="${COMP_WORDS[COMP_CWORD]}"
    opts=$( ${COMP_WORDS[@]:0:$COMP_CWORD} --generate-bash-completion )
    last=${COMP_WORDS[-2]}
    lastArg=${COMP_WORDS[-1]}
    case ${last} in
    --*)
        compopt -o default
        COMPREPLY=()
        ;;
    *)
        COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
        ;;
    esac
    return 0
}

complete -F _cli_bash_autocomplete $PROG
