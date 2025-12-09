flightctl login https://api.flightctlurl --insecure-skip-tls-verify --username <username> --password <password>

flightctl certificate request --signer=enrollment --expiration=365d --output=embedded > agentconfig.yaml

podman build -t quay.io/rcarroll/codeco/centos-bootc-flightctl:v2 -f TestDevice .

podman push quay.io/rcarroll/codeco/centos-bootc-flightctl:v2

mkdir -p output &&   sudo podman run --rm -it --privileged --pull=newer --security-opt label=type:unconfined_t     -v ${PWD}/output:/output -v /var/lib/containers/storage:/var/lib/containers/storage     quay.io/centos-bootc/bootc-image-builder:latest     --type raw quay.io/rcarroll/codeco/centos-bootc-flightctl:v2


Launch VM from image create in Output directory (e.g. with VirtManager or similar)