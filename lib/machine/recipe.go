package machine

const SWARM_MASTER = `version: "2"

services:
  swarm-master:
    image: swarm:1.2.3
    command: "manage -H tcp://0.0.0.0:2376 --tlsverify --tlscacert /conf/ca.pem --tlscert /conf/server-cert.pem -tlskey /conf/server-key.pem ${DISCOVERY_OPTS}"
    network_mode: host
    ports:
      - "2376:2376"
    volumes:
      - ${PWD}:/conf:ro
`

const COMPOSE = `---
provision:
- name: Install utility package
  action:
    - script: 00-install-pkg
      sudo: true

- name: Install Docker Engine
  action:
    - script: 01-install-docker-engine
      sudo: true

- name: Configure system setting
  action:
    - script: 02-config-system
      sudo: true

- name: Configure Docker Volume
  action:
    - cmd: 'pvcreate /dev/xvdb && vgcreate data /dev/xvdb && lvcreate -l 100%FREE -n docker data'
      shell: true
      sudo: true
    - cmd: 'mkfs.ext4 /dev/data/docker'
      sudo: true
    - cmd: 'echo "/dev/mapper/data-docker /data ext4 rw 0 0" >>/etc/fstab'
      shell: true
      sudo: true

- name: Configure Docker Engine
  archive:
    - src: docker.daemon.json
      dst: /etc/docker/daemon.json
      sudo: true
  action:
    - cmd: 'service docker stop'
      sudo: true
    - cmd: 'rm -f /etc/docker/key.json'
      sudo: true
    - cmd: 'rm -rf /var/lib/docker'
      sudo: true

---
provision:
- name: Configure swap
  action:
    - cmd: 'fallocate -l 8G /swapfile && chmod 600 /swapfile && mkswap /swapfile'
      shell: true
      sudo: true
    - cmd: 'echo "/swapfile none swap sw 0 0" >>/etc/fstab'
      shell: true
      sudo: true

---
provision:
- name: Clean up and Shutdown
  action:
    - cmd: shutdown -h now
      sudo: true
`

const INSTALL_PKG = `#!/bin/bash

# install common utility packages
apt-get update && apt-get upgrade -y && apt-get install -y \
    curl \
    htop \
    lvm2 \
    ntp \
    jq
`

const INSTALL_DOCKER_ENGINE = `#!/bin/bash

apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D
apt-get update && apt-get install -y apt-transport-https linux-image-extra-$(uname -r)
echo "deb https://apt.dockerproject.org/repo ubuntu-trusty main" | tee /etc/apt/sources.list.d/docker.list
apt-get update && apt-get install -y docker-engine
`

const CONFIGURE_SYSTEM = `#!/bin/bash

truncate -s0 /etc/sysctl.conf
echo "vm.overcommit_memory = 1" >>/etc/sysctl.conf

cat <<\EOF >/etc/init.d/disable-transparent-hugepages
#!/bin/sh
### BEGIN INIT INFO
# Provides:          disable-transparent-hugepages
# Required-Start:    $local_fs
# Required-Stop:
# X-Start-Before:    docker
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: Disable Linux transparent huge pages
# Description:       Disable Linux transparent huge pages, to improve
#                    database performance.
### END INIT INFO
case $1 in
start)
    if [ -d /sys/kernel/mm/transparent_hugepage ]; then
    thp_path=/sys/kernel/mm/transparent_hugepage
    elif [ -d /sys/kernel/mm/redhat_transparent_hugepage ]; then
    thp_path=/sys/kernel/mm/redhat_transparent_hugepage
    else
    return 0
    fi
    echo 'never' > ${thp_path}/enabled
    echo 'never' > ${thp_path}/defrag
    unset thp_path
    ;;
esac
EOF
chmod 755 /etc/init.d/disable-transparent-hugepages
update-rc.d disable-transparent-hugepages defaults

# Adjust server network limit
echo "net.ipv4.ip_local_port_range = 1024 65535" >>/etc/sysctl.conf
echo "net.ipv4.tcp_rmem = 4096 4096 16777216" >>/etc/sysctl.conf
echo "net.ipv4.tcp_wmem = 4096 4096 16777216" >>/etc/sysctl.conf
echo "net.ipv4.tcp_max_syn_backlog = 4096" >>/etc/sysctl.conf
echo "net.ipv4.tcp_syncookies = 1" >>/etc/sysctl.conf
echo "net.core.somaxconn = 1024" >>/etc/sysctl.conf

echo "fs.file-max = 100000" >>/etc/sysctl.conf
echo "* - nofile 100000" >>/etc/security/limits.conf
`

const DOCKER_DAEMON_CONFIG = `{
    "hosts": [
        "unix:///var/run/docker.sock"
    ],
    "graph": "/data"
}
`
