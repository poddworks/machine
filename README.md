# machine
Swiss Army knife for DevOps

## What is machine
**machine** supports DevOps workflow in two ways
- *Docker Engine* deployment
- *Ansible* like SSH orchestration

**machine** implements the following core functions
- create  
    Creates a *box* by the selected provider
- exec  
    Execute command, script, or playbook on remote targets

## Docker Engine Deployment
Execute `machine config <provider> sync` to populate settings from your
provider to local cache.

When **machine** provisions a *box* it assumes you are provisioning an instance
with Docker Engine and TLS enabled.  Default Linux Image is available for
your choice of provider, managed by me.

To review which Linux Image is used during provision, run
`machine config <provider> get`.

You are encouraged to build your own Linux Image by creating a *box* and then
register that image with that box for future provision.
- `machine create <provider> [options]`
- Install baseline environment to get you started on using the image as a
  deployment vehicle
- `machine register <provider> [options]`.
- Wait for the image to be created and available, this could take a few
  minutes.
- `machine config <provider> sync`

## Ansible like SSH orchestration
*Ansible* builds its deployment strategy around SSH.  **machine** does not
try to overtake Ansible; the need for  SSH orchestration is out of necessity.

*Docker Engine* is in the category of **agent** based deployment method, like
*Chef* and *Puppet*.  The first hurdle most people need to overcome is
provisioning a machine that runs *Docker Engine*.

There lies the question:
- How do you provision and configure a machine to run Docker?
- Without *SSH* to the machine and configure by hand.

The solution is instruct machine to run pre-configured scripts/commands via
*SSH*. **machine** provides this facility without users install yet another
tool for DevOps.

**machine** provides the following functions for SSH based orchestration:
- `machine exec run [cmd]`
  - Run a single command on remote hosts
- `machine exec script [user_script...]`
  - `user_script`s are `scp` to remote before execution.
  - Run `user_script`s on remote hosts.
- `machine exec playbook [compose.yml]`

A typical playbook config file looks like the following:
```yaml
archive:
  - 
    src: ./your-global-stuff.tgz
    dst: stuff.tgz
    dir: /tmp

provision:
  - 
    name: Unpack stuff
    action:
      - cmd: tar -zxvf /tmp/stuff.tgz -C /my/install/target
      - script: some-script-to-run
  - 
    name: Need to send more stuff and run something
    archive:
      - 
        src: ./more-stuff.tgz
        dir: /var/lib/my_stuff
        sudo: true
    action:
      - 
        cmd: tar -zxvf /var/lib/my_stuff/more-stuff.tgz -C /my/install/target
        sudo: true
      - 
        script: more-stuff
        sudo: true
```

## FAQ
- You know there is a tool called **docker-machine** right?  
    Yes, I do.  And you should use **docker-machine** where its applicable to
your DevOps workflow.
- It doesn't support X, Y, Z!  
    Feel free to send me pull requests to enable support for X, Y, Z.
- My thoughts on DevOps  
    I wrote this tool in the hopes that it will cover common problems in day to
day DevOps.  While my preferred deployment method is through Docker, enterprise
customer, especially ones with requirements in building on-premise datacenter,
may not appreciate the effort in maintaining on-premise Docker Registry and
Discovery service.  In these cases, SSH orchestration works perfectly with
minimal preperation and infrastructure requirements.
- My goal for **machine**  
    I hope that you find **machine** a compact Swiss Army knife for DevOps.
