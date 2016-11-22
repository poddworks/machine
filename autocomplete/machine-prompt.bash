__machine_ps1() {
    local format=${1:- (%s)}
    if test ${MACHINE_NAME}; then
        local status
        printf -- "${format}" "${MACHINE_NAME}${status}"
    fi
}
