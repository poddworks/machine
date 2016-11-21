#!/bin/bash

_machine_bash_autocomplete() {
    local cur opts base
    cur="${COMP_WORDS[COMP_CWORD]}"
    last=${COMP_WORDS[-2]}
    case ${last} in
    -*|script|playbook)
        compopt -o default
        COMPREPLY=()
        ;;
    use)
        COMPREPLY=( $(compgen -W "$(machine ls -q)" -- "${cur}") )
        ;;
    reboot|rm|start|stop)
        COMPREPLY=( $(compgen -W "$(machine ls -q)" -- "${cur}") )
        ;;
    *)
        opts=$( ${COMP_WORDS[@]:0:$COMP_CWORD} --generate-bash-completion )
        COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
        ;;
    esac
    return 0
}

complete -F _machine_bash_autocomplete machine
