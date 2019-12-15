This is a simple service allowing you to generate temporary SSH certificates in your Drone pipelines.

This require `DRONE_BUILD_SIGNED` and
`DRONE_REPO_SIGNED` environment variables, as defined by
[`drone-env-signed`](https://github.com/sloonz/drone-env-signed).

## Setup

First, generate a keypair for your SSH CA:

```console
ssh-keygen -t ed25519 -f id_ca
```

Download and run the service:

```console
$ docker run -d \
  --publish=3000:80 \
  --env=CA_DEBUG=true \
  --env=CA_ENV_PUBLIC_KEY="$CA_ENV_PUBLIC_KEY"
  --env=CA_PRIVATE_KEY="$(cat id_ca)" \
  --restart=always \
  --name=drone-env-merge
```

Where `CA_ENV_PUBLIC_KEY` is the public key associated to the private
key of the `drone-env-signed` plugin (in PEM format).

In the `~./ssh/authorized_keys` file of the target user, add the following line:

```text
cert-authority,principals="drone:sloonz/drone-ssh-ca:master" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEadp7QWtUaUGit9HxlSsvGCxcXJcZn1lC9rtaDkD/m5 drone-ssh-ca
```

Here, `ssh-ed25519 ... drone-ssh-ca` is the public key of your SSH CA (the content of `id_ca.pub` generated earlier).

`principals` is the allowed principals for this user. A certificate generated for the pipeline building the `sloonz/drone-ssh-ca` on `master` branch will have the following principals:

* drone
* drone:sloonz
* drone:sloonz/drone-ssh-ca
* drone:sloonz/drone-ssh-ca:master

Therefore, if you put `drone` in the `principals=` option of your
`authorized_keys` file, any of your pipeline will be allowed to connect
with SSH into this user. With `drone:sloonz`, any pipeline building
a project in the `sloonz` namespace will be allowed to connect. With
`drone:sloonz/drone-ssh-ca`, only buids for this project will be allowed
to connect. The full `drone:sloonz/drone-ssh-ca:master` also limits to
pipelines on the `master` branch of the project.

## Usage

```yaml
kind: pipeline
name: default
steps:
 - name: deploy
   image: sloonz/drone-ssh-client
   steps:
    - ssh-get-certificate http://drone-ssh-ca:3000
    - scp package.tar.gz user@example.com:htdocs/
   when:
    branch: master
```
