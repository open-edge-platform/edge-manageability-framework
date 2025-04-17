# Edge Orchestrator buildall automation

This set of makefile and scripts will rebuild all of the container images
and helm charts necessary to perform a deployment of the Edge Orchestrator.

To ensure that a tested Best Known Configuration set of components are used,
the versions rebuilt are derived from the versions specified in the current
checkout of the `edge-manageability-framework` repository.

## Prerequisites

Performing a buildall requires the following tools to be installed in a
Linux/Unix like environment:

- The gnu make build tool and bash shell
- Docker (both the local tool and the ability to build container images)
  - Must be Docker Engine 27.3.0 or newer which includes Docker BuildKit
  - The built docker images require around 5GB of space
  - Approximately roughly 50GB for build cache. See `DOCKER_PRUNE` below
    to optionally prune the build cache during the build process.
- Helm 3
- Python 3 (in-support version such as 3.10 or later, tested on 3.12)
- Mage

Additionally, some of the containers that are build require the following

- Open Policy Agent v0.69.0 (or possibly later) for infra-core repo

## Configuration

Configuration is done with environmental variables, and the following variables
should be set in the `env.sh` file:

- Docker variables: `DOCKER_REGISTRY`, `DOCKER_REPOSITORY`

Also, export any `PROXY_*` variables that may be needed if you are behind a
proxy.

In order to build docker images and refresh the repo list, please set:

- `GITHUB_TOKEN` to a [Github developer
  token](https://github.com/settings/tokens) to enable GitHub clone and API
  actions for updating the `repo_list` file

If you have limited space for docker images and want to clean up between each
docker build, set `DOCKER_PRUNE` to `Y`.

## Usage

All commands are run using `make`.

You can get a list of all the make targets with `make help`

To run the full process with the BKC from the currently checked out copy of
`edge-manageability-framework` repo, run `make buildall`.

To cleanup internal scratch state, run `make clean`

To fully cleanup (including tools), run `make clean-all`.  Note that this only
cleans up local files, not container images that were built.

### How it works

The `make buildall` target has many steps, which are described in the list
below:

1. All repos are checked out using `make checkout-repos`, per the list
   provided in the `repo_list` file. They are expected to be checked out from
   the `https://github.com/open-edge-platform` GitHub org.

2. The list of artifacts (helm charts, container images) that can be produced
   by each repo is obtained with `make list-artifacts`.

   This is done by calling `make helm-list` and `make docker-list` in each of
   the repos, which generates YAML data.

   `make helm-list` creates YAML data in this format.  Note that the `outDir`
   is optional and defaults to `.`:

   ```yaml
   charts:  # a dictionary of charts produced by this repo
      first-chart:  # name of the chart
         version: '1.0.0'  # version of the chart
         gitTagPrefix: 'first-chart-'  # the leading part of the git tag
         outDir: 'out'  # the output directory where the helm charts is created
      second-chart:  # another example chart, same format
         version: '1.5.0'
         gitTagPrefix: 'second-chart-'
         outDir: 'out'
   ```

   `make docker-list` creates YAML in this format. Note that `buildTarget` is
   optional and defaults to `docker-build`:

   ```yaml
   images:
     first-container:  # the base name of the image
       name: 'registry/repository/path/first-container:1.1.0'  # full image name
       version: '1.1.0'  # version of the image
       gitTagPrefix: 'first-container/v'   # the leading part of the git tag
       buildTarget: 'first-container-docker-build'  # make target to build
     second-container:  #  another example image, same format
       name: 'registry/repository/path/second-container:1.7.0'
       version: '1.7.0'
       gitTagPrefix: 'second-container/v'
       buildTarget: 'second-container-docker-build'
   ```

   Note that not all repos may generate charts or images - if either of the
   `helm-list` or `docker-list` are absent, they will not be called.

   The output of these commands is placed in `scratch/artifacts_<repo>.yaml`

3. The list of charts required by the checked out version of `orch-deploy` is
   determined by running `make chart-manifest`, which calls `mage
   gen:releaseManifest` in the parent directory, building a list of required
   charts from the BKC in the `edge-manageability-framework` repo.

   This is stored in `scratch/manifest_charts.yaml`

4. The charts provided by each repo and manifest are compared with `make
   sort-charts`, which generates a list of tags required to be checked out and
   built in every repo that contains helm charts.

   This creates multiple `scratch/htags_<repo>` files

5. The charts are then built by using the `make helm-build`, which checks out
   the tag in each repo and builds chart .tgz files. These charts are then
   copied into the `charts` temp directory.

   The output of this is stored in `scratch/hbuild_<repo>` files, which contain
   the log of the build process in that repo.

6. Now that charts have been created, `make image-manifest` is run, which calls
   `mage gen:localReleaseImageManifest`. This uses the BKC in the
   `edge-manageability-framework` repo and the charts that were build in the
   previous step to generate a list of container images that need to be rebuilt.

   This information is stored in `scratch/manifest_images.yaml`

7. With the list of required images, `make sort-images` is run, which creates
   parses the artifact lists from each repo and determines which images to
   build in each. It then generates per-repo lists

   This creates multiple `scratch/itags_<repo>` files

8. Now that repos and tags are know, the images are rebuilt with `make
   image-build`, which checks out the tag in each repo, and runs either the
   `make <buildTarget>` specified for the container image, or lacking that
   `make docker-image` in each repo. The containers are stored in the local
   docker image cache.

   This creates output in `scratch/ibuild_<repo>` files, which contain the log
   of the build process in that repo.

### Troubleshooting

#### Failure in checkout-repos step

Check that you have http access to GitHub to pull repos.

#### Failure in list-artifacts step

You may not have the right branches checked out of each repo - try running
`make branch-checkout`, which will checkout the `main` branch and pull any
recent changes. If this target fails, there may be changes in the repo - go
into `repos/<repo>` to investigate.

### Performance

The most performance intensive steps are building the artifact lists, the helm
charts, and especially the docker images.  Fortunately with the design of this
system, they can all be run in parallel across all repos at the same time - the
level of parallelism can be configured with the `MAKE_JOBS` in `env.sh`.

With this enabled, you can expect a modest (25-40%) reduction in docker image
build times, depending on the compute and network resources available on the
machine.

Within each repo the process is serialized as only a single tag can be checked
out in a repo at a time.

Building additional times will leverage both the ability of make to keep track
of progress (via files in scratch), and the docker cache.
