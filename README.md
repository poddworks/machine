# machine
Swiss Army knife for DevOps

## What is machine
**machine** supports DevOps workflow in two ways
- *Provision Virtual Machine* through provider
- *Docker Engine* deployment
- *Ansible* like SSH orchestration

I wrote this tool in the hopes that it will cover common problems in day to
day DevOps.  While my preferred deployment method is through Docker, enterprise
customer, especially ones with requirements in building on-premise data center,
may not appreciate the effort in maintaining on-premise Docker Registry and
Discovery service.  In these cases, SSH orchestration works perfectly with
less preperation and infrastructure requirements.

## Provision Virtual Machine
Execute `machine <provider> config sync` to populate settings from your
provider to local cache.

To review which Linux Image is used during provision, run
`machine <provider> config get`.

Finally, create VM through provider.  Please follow the instructions provided
by provider for the meaning behind each option.  Supported providers are:
- AWS

## Docker Engine Deployment
By default Virtual Machine will be provisioned without Docker Engine Installed.
To install Docker Engine and make Docker Host remote acceessible, turn on
`--use-docker` flag for each provider.  Target machine is assumed to be a
Ubuntu/Debian system. See [SSH orchestration](#ansible-linke-ssh-orchestration)
on how to deploy Docker Engine on other host operating system.

During provision, for each started instance, the program deploys Docker Engine
and a Self-Signed certificate.  On completion, Docker will be reacable
at `tcp:2376`.  Certificates are installed at the default location
`~/.machine`.

Docker Engine Deployment is used to **plan your deployment**.  Using
**machine** in production would require users to procure *CA certificate* from
**trusted authoratative source** to prevent MITM attack (Man in the Middle).

## Ansible like SSH orchestration
*Ansible* builds its deployment strategy around SSH.  **machine** does not
try to overtake Ansible; the need for SSH orchestration is out of necessity.

*Docker Engine* is in the category of **agent** based deployment method, like
*Chef* and *Puppet*.  The first hurdle most people need to overcome is
provisioning a machine that runs *Docker Engine*.

There lies the question:
- How do you provision and configure a machine to run Docker?
- Without *SSH* into the machine and configure by hand.

The solution is to instruct instance to run pre-configured scripts/commands via
*SSH*. **machine** provides this facility without users install yet another
tool for DevOps.

A typical playbook config file looks like the following:
```yaml
archive:
- src: ./your-global-stuff.tgz
  dst: stuff.tgz
  dir: /tmp

provision:
- name: Unpack stuff
  action:
    - cmd: tar -zxvf /tmp/stuff.tgz -C /my/install/target
    - script: some-script-to-run
- name: Need to send more stuff and run something
  archive:
    - src: ./more-stuff.tgz
      dir: /var/lib/my_stuff
      sudo: true
  action:
    - cmd: tar -zxvf /var/lib/my_stuff/more-stuff.tgz -C /my/install/target
      sudo: true
    - script: more-stuff
      sudo: true
```

A recipe for how to build an instance into a working Docker Engine can be
generated through `gen-recipe` command.  This will produce the following items:
- compose.yml
- 00-install-pkg
- 01-install-docker-engine
- 02-config-system
- docker.daemon.json

Execute `machine exec --host <instance_hostname> playbook compose.yml` to
complete provisioning.

## Appendix - Command reference
```
NAME:
   machine - Swiss Army knife for DevOps

USAGE:
   machine [global options] command [command options] [arguments...]

VERSION:
   1.0.0

AUTHOR(S):
   Yi-Hung Jen <yihungjen@gmail.com>

COMMANDS:
     create   Create instances
     start    Start instances
     stop     Stop instances
     reboot   Reboot instances
     rm       Remove And Terminate instances
     ls       List cached Docker Engine instance info
     ip       Obtain IP address of the Docker Engine instance
     env      Apply Docker Engine environment for target
     exec     Invoke command on remote host via SSH
     ssh      Login to remote machine with SSH
     tls      Generate certificate for TLS
     dns      Query DNS record
     aws      Manage resources on AWS
     recipe   Generate recipe for provision/management
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --user value     Run command as user [$MACHINE_USER]
   --cert value     Private key to use in Authentication [$MACHINE_CERT_FILE]
   --port value     Connected to ssh port (default: "22") [$MACHINE_PORT]
   --org value      Organization for Self Signed CA (default: "podd.org")
   --confdir value  Configuration and Certificate path (default: "~/.machine")
   --help, -h       show help
   --version, -v    print the version
```
