# terraclaw

Go-based interactive CLI for converting existing cloud resources to Terraform/OpenTofu configuration using AI.

> **Looking to codify at scale?** If you need to codify cloud resources across your organization with minimal setup — plus governance, drift detection, and module management — check out
> <a href="https://www.stackguardian.io/platform/code"><svg width="10em" height="1.8em" viewBox="0 0 202 36" fill="none" xmlns="http://www.w3.org/2000/svg" style="vertical-align: middle;"><rect width="36" height="36" rx="18" fill="#5B44F2"></rect><path fill-rule="evenodd" clip-rule="evenodd" d="M6 18C6 11.3726 11.3726 6 18 6C24.6274 6 30 11.3726 30 18C30 24.6274 24.6274 30 18 30C11.3726 30 6 24.6274 6 18Z" stroke="white" stroke-width="1.75"></path><path fill-rule="evenodd" clip-rule="evenodd" d="M12 18C12 14.6863 14.6863 12 18 12C21.3137 12 24 14.6863 24 18C24 21.3137 21.3137 24 18 24C14.6863 24 12 21.3137 12 18Z" stroke="white" stroke-width="2.5"></path><path d="M50.208 26.692C49.072 26.692 48 26.524 46.992 26.188C46 25.836 45.168 25.284 44.496 24.532C43.824 23.764 43.384 22.764 43.176 21.532H46.224C46.416 22.156 46.72 22.644 47.136 22.996C47.568 23.348 48.072 23.596 48.648 23.74C49.224 23.868 49.808 23.932 50.4 23.932C50.928 23.932 51.424 23.868 51.888 23.74C52.368 23.596 52.76 23.364 53.064 23.044C53.384 22.724 53.544 22.3 53.544 21.772C53.544 21.372 53.456 21.052 53.28 20.812C53.12 20.556 52.888 20.348 52.584 20.188C52.296 20.012 51.944 19.876 51.528 19.78C51.128 19.652 50.696 19.54 50.232 19.444C49.768 19.348 49.312 19.244 48.864 19.132C48.416 19.02 47.992 18.876 47.592 18.7C47.08 18.524 46.584 18.324 46.104 18.1C45.64 17.86 45.232 17.572 44.88 17.236C44.544 16.9 44.272 16.484 44.064 15.988C43.856 15.492 43.752 14.9 43.752 14.212C43.752 13.428 43.88 12.756 44.136 12.196C44.408 11.62 44.76 11.148 45.192 10.78C45.64 10.412 46.136 10.124 46.68 9.916C47.24 9.708 47.816 9.556 48.408 9.46C49.016 9.364 49.592 9.316 50.136 9.316C51.192 9.316 52.152 9.492 53.016 9.844C53.896 10.196 54.616 10.74 55.176 11.476C55.736 12.212 56.056 13.156 56.136 14.308H53.232C53.168 13.764 52.976 13.332 52.656 13.012C52.336 12.676 51.936 12.436 51.456 12.292C50.976 12.132 50.456 12.052 49.896 12.052C49.512 12.052 49.128 12.084 48.744 12.148C48.376 12.212 48.04 12.324 47.736 12.484C47.448 12.644 47.208 12.86 47.016 13.132C46.84 13.388 46.752 13.716 46.752 14.116C46.752 14.468 46.84 14.78 47.016 15.052C47.192 15.308 47.432 15.524 47.736 15.7C48.056 15.876 48.432 16.036 48.864 16.18C49.424 16.404 50.032 16.572 50.688 16.684C51.36 16.796 51.992 16.948 52.584 17.14C53.16 17.3 53.688 17.5 54.168 17.74C54.664 17.964 55.088 18.244 55.44 18.58C55.792 18.9 56.064 19.3 56.256 19.78C56.464 20.244 56.568 20.804 56.568 21.46C56.568 22.42 56.392 23.236 56.04 23.908C55.704 24.564 55.232 25.1 54.624 25.516C54.032 25.932 53.352 26.236 52.584 26.428C51.832 26.604 51.04 26.692 50.208 26.692ZM63.5094 26.644C62.1174 26.644 61.1014 26.26 60.4614 25.492C59.8214 24.724 59.5014 23.66 59.5014 22.3V16.444H57.5574V13.852H59.5014V10.3H62.4294V13.852H66.2694V16.444H62.4294V21.748C62.4294 22.164 62.4774 22.54 62.5734 22.876C62.6694 23.212 62.8454 23.476 63.1014 23.668C63.3574 23.86 63.7094 23.964 64.1574 23.98C64.5414 23.98 64.8694 23.916 65.1414 23.788C65.4134 23.66 65.6374 23.516 65.8134 23.356L66.7974 25.588C66.4934 25.844 66.1494 26.052 65.7654 26.212C65.3974 26.372 65.0214 26.484 64.6374 26.548C64.2534 26.612 63.8774 26.644 63.5094 26.644ZM71.6955 26.668C71.1035 26.668 70.5435 26.588 70.0155 26.428C69.4875 26.268 69.0155 26.036 68.5995 25.732C68.1835 25.412 67.8475 25.02 67.5915 24.556C67.3515 24.076 67.2315 23.516 67.2315 22.876C67.2315 22.06 67.3915 21.396 67.7115 20.884C68.0315 20.356 68.4635 19.956 69.0075 19.684C69.5675 19.412 70.1995 19.228 70.9035 19.132C71.6075 19.02 72.3435 18.964 73.1115 18.964H75.5835C75.5835 18.404 75.4955 17.924 75.3195 17.524C75.1435 17.108 74.8635 16.78 74.4795 16.54C74.1115 16.3 73.6315 16.18 73.0395 16.18C72.6555 16.18 72.2955 16.228 71.9595 16.324C71.6395 16.404 71.3595 16.54 71.1195 16.732C70.8955 16.908 70.7355 17.14 70.6395 17.428H67.6395C67.7515 16.772 67.9755 16.212 68.3115 15.748C68.6635 15.268 69.0875 14.876 69.5835 14.572C70.0955 14.268 70.6555 14.044 71.2635 13.9C71.8715 13.74 72.4875 13.66 73.1115 13.66C74.9515 13.66 76.2875 14.188 77.1195 15.244C77.9675 16.3 78.3915 17.78 78.3915 19.684V26.5H75.7995L75.7275 24.868C75.3595 25.412 74.9195 25.812 74.4075 26.068C73.8955 26.324 73.3995 26.484 72.9195 26.548C72.4395 26.628 72.0315 26.668 71.6955 26.668ZM72.2715 24.292C72.9115 24.292 73.4795 24.172 73.9755 23.932C74.4715 23.692 74.8635 23.356 75.1515 22.924C75.4555 22.492 75.6075 21.996 75.6075 21.436V21.028H73.3035C72.9195 21.028 72.5355 21.044 72.1515 21.076C71.7835 21.092 71.4475 21.156 71.1435 21.268C70.8395 21.364 70.5915 21.516 70.3995 21.724C70.2235 21.932 70.1355 22.228 70.1355 22.612C70.1355 22.996 70.2315 23.308 70.4235 23.548C70.6315 23.788 70.8955 23.972 71.2155 24.1C71.5515 24.228 71.9035 24.292 72.2715 24.292ZM86.0349 26.668C84.7229 26.668 83.6189 26.404 82.7229 25.876C81.8269 25.332 81.1469 24.572 80.6829 23.596C80.2189 22.62 79.9869 21.492 79.9869 20.212C79.9869 18.932 80.2269 17.804 80.7069 16.828C81.1869 15.836 81.8909 15.06 82.8189 14.5C83.7469 13.94 84.8749 13.66 86.2029 13.66C87.1789 13.66 88.0429 13.828 88.7949 14.164C89.5629 14.5 90.1789 14.996 90.6429 15.652C91.1229 16.292 91.4269 17.092 91.5549 18.052H88.6749C88.4829 17.444 88.1629 17.004 87.7149 16.732C87.2829 16.444 86.7629 16.3 86.1549 16.3C85.3389 16.3 84.6909 16.5 84.2109 16.9C83.7469 17.284 83.4109 17.772 83.2029 18.364C82.9949 18.956 82.8909 19.572 82.8909 20.212C82.8909 20.884 83.0029 21.516 83.2269 22.108C83.4509 22.684 83.7949 23.148 84.2589 23.5C84.7389 23.852 85.3549 24.028 86.1069 24.028C86.7149 24.028 87.2589 23.892 87.7389 23.62C88.2349 23.348 88.5549 22.908 88.6989 22.3H91.6749C91.5629 23.276 91.2349 24.092 90.6909 24.748C90.1629 25.388 89.4909 25.868 88.6749 26.188C87.8589 26.508 86.9789 26.668 86.0349 26.668ZM93.1532 26.5V9.532H96.0812V18.916H97.9052L101.241 13.852H104.529L100.425 19.876L104.865 26.5H101.457L98.0732 21.484H96.0812V26.5H93.1532ZM113.119 26.692C111.391 26.692 109.927 26.332 108.727 25.612C107.527 24.876 106.615 23.86 105.991 22.564C105.383 21.252 105.079 19.74 105.079 18.028C105.079 16.732 105.247 15.556 105.583 14.5C105.935 13.444 106.447 12.532 107.119 11.764C107.807 10.98 108.647 10.38 109.639 9.964C110.631 9.532 111.775 9.316 113.071 9.316C114.335 9.316 115.463 9.524 116.455 9.94C117.463 10.34 118.287 10.932 118.927 11.716C119.583 12.5 120.023 13.46 120.247 14.596H117.151C117.023 14.052 116.775 13.596 116.407 13.228C116.055 12.86 115.607 12.58 115.063 12.388C114.519 12.196 113.895 12.1 113.191 12.1C112.279 12.1 111.503 12.26 110.863 12.58C110.223 12.9 109.703 13.34 109.303 13.9C108.903 14.444 108.607 15.076 108.415 15.796C108.239 16.5 108.151 17.252 108.151 18.052C108.151 19.092 108.319 20.06 108.655 20.956C109.007 21.852 109.551 22.572 110.287 23.116C111.039 23.644 112.015 23.908 113.215 23.908C114.063 23.908 114.807 23.764 115.447 23.476C116.103 23.188 116.615 22.772 116.983 22.228C117.367 21.668 117.575 20.98 117.607 20.164H112.663V17.572H120.631V18.676C120.631 20.324 120.343 21.748 119.767 22.948C119.191 24.148 118.343 25.076 117.223 25.732C116.119 26.372 114.751 26.692 113.119 26.692ZM127.868 26.668C126.044 26.668 124.644 26.196 123.668 25.252C122.708 24.308 122.228 22.868 122.228 20.932V13.852H125.156V20.74C125.156 21.412 125.244 21.996 125.42 22.492C125.612 22.988 125.908 23.372 126.308 23.644C126.724 23.9 127.244 24.028 127.868 24.028C128.54 24.028 129.068 23.892 129.452 23.62C129.852 23.332 130.132 22.94 130.292 22.444C130.452 21.948 130.532 21.38 130.532 20.74V13.852H133.46V20.932C133.46 22.916 132.964 24.372 131.972 25.3C130.996 26.212 129.628 26.668 127.868 26.668ZM139.556 26.668C138.964 26.668 138.404 26.588 137.876 26.428C137.348 26.268 136.876 26.036 136.46 25.732C136.044 25.412 135.708 25.02 135.452 24.556C135.212 24.076 135.092 23.516 135.092 22.876C135.092 22.06 135.252 21.396 135.572 20.884C135.892 20.356 136.324 19.956 136.868 19.684C137.428 19.412 138.06 19.228 138.764 19.132C139.468 19.02 140.204 18.964 140.972 18.964H143.444C143.444 18.404 143.356 17.924 143.18 17.524C143.004 17.108 142.724 16.78 142.34 16.54C141.972 16.3 141.492 16.18 140.9 16.18C140.516 16.18 140.156 16.228 139.82 16.324C139.5 16.404 139.22 16.54 138.98 16.732C138.756 16.908 138.596 17.14 138.5 17.428H135.5C135.612 16.772 135.836 16.212 136.172 15.748C136.524 15.268 136.948 14.876 137.444 14.572C137.956 14.268 138.516 14.044 139.124 13.9C139.732 13.74 140.348 13.66 140.972 13.66C142.812 13.66 144.148 14.188 144.98 15.244C145.828 16.3 146.252 17.78 146.252 19.684V26.5H143.66L143.588 24.868C143.22 25.412 142.78 25.812 142.268 26.068C141.756 26.324 141.26 26.484 140.78 26.548C140.3 26.628 139.892 26.668 139.556 26.668ZM140.132 24.292C140.772 24.292 141.34 24.172 141.836 23.932C142.332 23.692 142.724 23.356 143.012 22.924C143.316 22.492 143.468 21.996 143.468 21.436V21.028H141.164C140.78 21.028 140.396 21.044 140.012 21.076C139.644 21.092 139.308 21.156 139.004 21.268C138.7 21.364 138.452 21.516 138.26 21.724C138.084 21.932 137.996 22.228 137.996 22.612C137.996 22.996 138.092 23.308 138.284 23.548C138.492 23.788 138.756 23.972 139.076 24.1C139.412 24.228 139.764 24.292 140.132 24.292ZM148.328 26.5V13.852H150.992L151.16 15.508C151.48 15.028 151.84 14.66 152.24 14.404C152.656 14.132 153.096 13.94 153.56 13.828C154.04 13.716 154.52 13.66 155 13.66C155.176 13.66 155.336 13.66 155.48 13.66C155.64 13.66 155.768 13.668 155.864 13.684V16.468H155.096C154.264 16.468 153.56 16.628 152.984 16.948C152.408 17.268 151.976 17.724 151.688 18.316C151.4 18.908 151.256 19.636 151.256 20.5V26.5H148.328ZM161.791 26.668C160.511 26.668 159.455 26.38 158.623 25.804C157.791 25.212 157.167 24.428 156.751 23.452C156.351 22.476 156.151 21.388 156.151 20.188C156.151 18.94 156.367 17.828 156.799 16.852C157.231 15.86 157.879 15.084 158.743 14.524C159.607 13.948 160.687 13.66 161.983 13.66C162.463 13.66 162.927 13.716 163.375 13.828C163.839 13.924 164.271 14.084 164.671 14.308C165.087 14.516 165.431 14.796 165.703 15.148V9.532H168.631V26.5H165.799L165.751 24.916C165.447 25.316 165.087 25.644 164.671 25.9C164.271 26.156 163.823 26.348 163.327 26.476C162.831 26.604 162.319 26.668 161.791 26.668ZM162.343 24.028C163.111 24.028 163.743 23.852 164.239 23.5C164.735 23.132 165.103 22.652 165.343 22.06C165.599 21.452 165.727 20.812 165.727 20.14C165.727 19.436 165.599 18.796 165.343 18.22C165.103 17.644 164.735 17.18 164.239 16.828C163.743 16.476 163.111 16.3 162.343 16.3C161.527 16.3 160.879 16.484 160.399 16.852C159.935 17.22 159.591 17.708 159.367 18.316C159.159 18.908 159.055 19.548 159.055 20.236C159.055 20.748 159.111 21.236 159.223 21.7C159.351 22.148 159.543 22.548 159.799 22.9C160.055 23.252 160.391 23.532 160.807 23.74C161.223 23.932 161.735 24.028 162.343 24.028ZM170.747 26.5V13.852H173.675V26.5H170.747ZM172.211 12.388C171.651 12.388 171.203 12.22 170.867 11.884C170.531 11.548 170.363 11.108 170.363 10.564C170.363 10.036 170.539 9.604 170.891 9.268C171.243 8.916 171.683 8.74 172.211 8.74C172.723 8.74 173.163 8.908 173.531 9.244C173.899 9.58 174.083 10.02 174.083 10.564C174.083 11.108 173.907 11.548 173.555 11.884C173.203 12.22 172.755 12.388 172.211 12.388ZM179.825 26.668C179.233 26.668 178.673 26.588 178.145 26.428C177.617 26.268 177.145 26.036 176.729 25.732C176.313 25.412 175.977 25.02 175.721 24.556C175.481 24.076 175.361 23.516 175.361 22.876C175.361 22.06 175.521 21.396 175.841 20.884C176.161 20.356 176.593 19.956 177.137 19.684C177.697 19.412 178.329 19.228 179.033 19.132C179.737 19.02 180.473 18.964 181.241 18.964H183.713C183.713 18.404 183.625 17.924 183.449 17.524C183.273 17.108 182.993 16.78 182.609 16.54C182.241 16.3 181.761 16.18 181.169 16.18C180.785 16.18 180.425 16.228 180.089 16.324C179.769 16.404 179.489 16.54 179.249 16.732C179.025 16.908 178.865 17.14 178.769 17.428H175.769C175.881 16.772 176.105 16.212 176.441 15.748C176.793 15.268 177.217 14.876 177.713 14.572C178.225 14.268 178.785 14.044 179.393 13.9C180.001 13.74 180.617 13.66 181.241 13.66C183.081 13.66 184.417 14.188 185.249 15.244C186.097 16.3 186.521 17.78 186.521 19.684V26.5H183.929L183.857 24.868C183.489 25.412 183.049 25.812 182.537 26.068C182.025 26.324 181.529 26.484 181.049 26.548C180.569 26.628 180.161 26.668 179.825 26.668ZM180.401 24.292C181.041 24.292 181.609 24.172 182.105 23.932C182.601 23.692 182.993 23.356 183.281 22.924C183.585 22.492 183.737 21.996 183.737 21.436V21.028H181.433C181.049 21.028 180.665 21.044 180.281 21.076C179.913 21.092 179.577 21.156 179.273 21.268C178.969 21.364 178.721 21.516 178.529 21.724C178.353 21.932 178.265 22.228 178.265 22.612C178.265 22.996 178.361 23.308 178.553 23.548C178.761 23.788 179.025 23.972 179.345 24.1C179.681 24.228 180.033 24.292 180.401 24.292ZM188.596 26.5V13.852H191.332L191.5 15.46C191.836 15.012 192.228 14.66 192.676 14.404C193.124 14.148 193.596 13.964 194.092 13.852C194.588 13.724 195.052 13.66 195.484 13.66C196.684 13.66 197.628 13.924 198.316 14.452C199.02 14.98 199.516 15.684 199.804 16.564C200.092 17.444 200.236 18.428 200.236 19.516V26.5H197.308V19.996C197.308 19.532 197.276 19.084 197.212 18.652C197.148 18.204 197.02 17.804 196.828 17.452C196.652 17.1 196.396 16.82 196.06 16.612C195.724 16.404 195.276 16.3 194.716 16.3C194.028 16.3 193.444 16.484 192.964 16.852C192.484 17.22 192.124 17.716 191.884 18.34C191.644 18.948 191.524 19.644 191.524 20.428V26.5H188.596Z" fill="currentColor"></path></svg> Code</a>.

## Overview

`terraclaw` is a [BubbleTea](https://github.com/charmbracelet/bubbletea)-powered terminal UI tool that:

1. **Discovers** your existing cloud resources via [Steampipe](https://steampipe.io/) (AWS and Azure)
2. **Lets you select** the resources you want to import interactively
3. **Matches your own Terraform modules** from git repos or local paths, scoring them by fit
4. **Generates Terraform HCL** using [OpenCode](https://opencode.ai/) (AI coding agent), preferring your modules over public registry modules
5. **Runs `terraform import`** to create state files for the selected resources

## Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [Steampipe](https://steampipe.io/downloads) with at least one cloud plugin installed
- [Terraform](https://developer.hashicorp.com/terraform/downloads) (for the import step)
- [OpenCode](https://opencode.ai/) (`brew install opencode` or `npm install -g opencode-ai`)
- An AI provider configured in OpenCode (e.g. GitHub Copilot, OpenAI) via `opencode providers login`

## Installation

```bash
git clone https://github.com/arunim2405/terraclaw.git
cd terraclaw
make build
```

Or install directly:

```bash
go install .
```

## Configuration

`terraclaw` reads configuration from environment variables or a `.env` file in the working directory. The AI provider is configured through OpenCode's own auth system, not via terraclaw env vars.

| Variable             | Default       | Description                                      |
|----------------------|---------------|--------------------------------------------------|
| `STEAMPIPE_HOST`     | `localhost`   | Steampipe PostgreSQL host                        |
| `STEAMPIPE_PORT`     | `9193`        | Steampipe PostgreSQL port                        |
| `STEAMPIPE_DB`       | `steampipe`   | Steampipe database name                          |
| `STEAMPIPE_USER`     | `steampipe`   | Steampipe database user                          |
| `STEAMPIPE_PASSWORD` | _(empty)_     | Steampipe database password                      |
| `OPENCODE_PORT`      | `4096`        | Port for the OpenCode headless server            |
| `TERRAFORM_BIN`      | `terraform`   | Path to the `terraform` binary                   |
| `OUTPUT_DIR`         | `./output`    | Directory to write generated `.tf` files         |
| `CACHE_DIR`          | `~/.cache/terraclaw` | Directory for SQLite scan cache           |
| `CACHE_TTL`          | `1h`          | How long cached scans remain valid               |
| `NO_CACHE`           | `false`       | Skip the resource cache entirely                 |
| `DEBUG`              | `false`       | Enable debug logging to file                     |
| `DEBUG_LOG_FILE`     | `terraclaw.log` | Path for debug log output                      |

## OpenCode Setup

terraclaw delegates all LLM interaction to [OpenCode](https://opencode.ai/), which manages its own provider authentication. To get started:

```bash
# Log in to a provider (e.g. GitHub Copilot, OpenAI)
opencode providers login

# Verify your credentials
opencode providers list
```

The project includes an `opencode.json` that configures the model, permissions, and Terraform skills for headless operation. OpenCode will automatically pick it up from the project directory.

## Usage

### Interactive mode (TUI)

1. Start Steampipe with your desired cloud plugin:

```bash
steampipe plugin install aws
steampipe service start
```

2. Run `terraclaw`:

```bash
terraclaw
```

3. Follow the interactive prompts:
   - Select a cloud provider (Steampipe schema)
   - Select a resource type (table)
   - Toggle individual resources with **Space**, confirm with **Enter**
   - Review the generated Terraform code
   - Confirm running `terraform import`

### Non-interactive mode

Generate Terraform directly by specifying resource ARNs or Azure resource IDs:

```bash
# AWS
terraclaw generate --resources arn:aws:s3:::my-bucket --schema aws

# Azure
terraclaw generate --resources /subscriptions/xxx/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm1 --schema azure

# With user modules (auto-select all matching modules)
terraclaw generate --resources arn:aws:s3:::my-bucket --schema aws --auto-modules
```

### User Modules

Register your organization's Terraform modules so terraclaw uses them instead of public registry modules during code generation.

#### Adding a module

```bash
# From a git repository (Terraform source format)
terraclaw add-module "git::https://github.com/acme/tf-modules.git//modules/vpc?ref=v2.0"

# From a git repo via SSH
terraclaw add-module "git@github.com:acme/tf-modules.git//modules/vpc?ref=v2.0"

# From a local directory
terraclaw add-module ./my-modules/vpc

# With a custom name (overrides auto-derived name)
terraclaw add-module --name "custom-vpc" "git::https://github.com/acme/tf-modules.git//modules/vpc?ref=v2.0"
```

When you add a module, terraclaw automatically:
- Clones the repository (for git sources) or reads the local directory
- Parses all `.tf` files to extract resource types, variables, and outputs
- Detects the cloud provider (AWS, Azure) from resource type prefixes
- Reads the README for a description
- Stores the metadata in a local SQLite database (`~/.cache/terraclaw/modules.db`)

#### Managing modules

```bash
# List all registered modules
terraclaw list-modules

# Show full metadata (variables, outputs, resource types)
terraclaw inspect-module vpc

# Remove a module
terraclaw remove-module vpc
```

#### How modules are used during generation

**Interactive mode (TUI):** After you select resources and confirm generation, terraclaw checks your registered modules for matches. If any modules manage the same resource types you selected, a module selection screen appears showing each module's **fit score** (0-100%). Modules scoring 60%+ are pre-selected. Toggle with **Space**, confirm with **Enter**.

**Non-interactive mode:** Use `--use-modules` or `--auto-modules`:

```bash
# Auto-select all matching modules
terraclaw generate --resources arn:aws:... --schema aws --auto-modules

# Use modules (pre-selects those with >= 60% fit)
terraclaw generate --resources arn:aws:... --schema aws --use-modules
```

#### Fit score

The fit score tells you how well a module matches your selected resources:

| Component | Weight | What it measures |
|-----------|--------|------------------|
| Coverage | 50% | Fraction of your selected resource types the module handles |
| Specificity | 30% | How focused the module is (penalizes modules that drag in many unrelated resources) |
| Variable Readiness | 20% | Fraction of required variables that have defaults or match resource properties |

Selected modules are injected into the AI prompt as hard constraints, taking priority over public registry modules (e.g. `terraform-aws-modules`).

### Doctor checks

Validate local dependencies and configuration:

```bash
terraclaw doctor
```

This checks that `steampipe`, `terraform`, and `opencode` are available on `PATH`, the output directory is writable, and Steampipe is reachable.

### Flags

**Global flags:**
```
--output-dir string      Directory to write generated Terraform files (default "./output")
--terraform-bin string   Path to the terraform binary (default "terraform")
--debug                  Enable debug logging to file (see DEBUG_LOG_FILE)
--no-cache               Skip the resource cache and always rescan from Steampipe
```

**Generate flags:**
```
-r, --resources string   Comma-separated resource ARNs or Azure resource IDs (required)
    --schema string      Steampipe schema (auto-detected from resource IDs if omitted)
    --use-modules        Use registered user modules (matched by resource type)
    --auto-modules       Auto-select all matching user modules (implies --use-modules)
```

**Add-module flags:**
```
    --name string        Override the auto-derived module name
```

## Docker

A Docker image bundles all dependencies (Steampipe, Terraform, OpenCode, AWS CLI) for self-contained runs.

### Build

```bash
make docker-build
```

### Run

1. Create your env file from the sample:

```bash
make env-sample
# Edit .docker.env with your credentials
```

2. Run the container:

```bash
make docker-run
```

Or extract output artifacts to a local directory:

```bash
make docker-run-artifacts
```

The `.sample.docker.env` documents all required and optional environment variables for Docker runs. Key variables:

| Variable             | Required | Description                                      |
|----------------------|----------|--------------------------------------------------|
| `TERRACLAW_CMD_B64`  | Yes      | Base64-encoded terraclaw CLI command              |
| `OPENCLAW_CREDS_B64` | Yes      | Base64-encoded OpenCode `auth.json` content       |
| `AWS_ACCESS_KEY_ID`  | No       | AWS credentials (auto-detected by Steampipe)      |
| `AWS_SECRET_ACCESS_KEY` | No    | AWS secret key                                    |
| `AWS_REGION`         | No       | AWS region                                        |
| `LOCAL_ARTIFACTS_DIR`| No       | Bind-mount path to copy the output zip into       |
| `STEAMPIPE_PLUGINS`  | No       | Comma-separated extra plugins (e.g. `azure,gcp`)  |

### Generating `OPENCLAW_CREDS_B64`

This variable contains your OpenCode auth credentials (provider tokens) base64-encoded so they can be injected into the container at runtime. The credentials live in OpenCode's local auth file on your machine.

1. First, make sure you have at least one provider logged in locally:

```bash
opencode providers login   # follow the OAuth flow for GitHub Copilot, OpenAI, etc.
opencode providers list    # verify credentials are saved
```

2. Base64-encode the auth file:

```bash
# macOS
cat ~/.local/share/opencode/auth.json | base64

# Linux
cat ~/.local/share/opencode/auth.json | base64 -w 0
```

3. Paste the output into your `.docker.env`:

```env
OPENCLAW_CREDS_B64=eyJnaXRodWItY29waWxvdCI6ey...
```

Similarly, for `TERRACLAW_CMD_B64`:

```bash
echo "terraclaw generate --resources arn:aws:s3:::my-bucket --schema aws" | base64
```

### What happens inside the container

The entrypoint (`docker/entrypoint.sh`) runs these steps in order:

1. Writes OpenCode auth credentials from `OPENCLAW_CREDS_B64`
2. Installs Steampipe plugins and starts the Steampipe service
3. Starts `opencode serve` in the background
4. Runs the decoded terraclaw command
5. Zips the output and optionally copies it to `LOCAL_ARTIFACTS_DIR`
6. Cleans up OpenCode and Steampipe processes

## Makefile

Run `make help` to see all available targets:

```
build                  Build the binary for the current platform
build-linux            Cross-compile for linux/amd64
run                    Build and run with sample args (override with ARGS=)
test                   Run all tests
lint                   Run go vet
clean                  Remove build artifacts
docker-build           Build the Docker image
docker-run             Run the Docker container with .docker.env
docker-run-artifacts   Run and extract output to ./artifacts
docker-shell           Open a shell in the Docker container
env-sample             Copy sample env to .docker.env
help                   Show this help
```

## Project Structure

```
terraclaw/
├── main.go                         Entry point
├── cmd/
│   ├── root.go                     Cobra CLI setup & TUI entry
│   ├── generate.go                 Non-interactive generate command
│   ├── module.go                   Module management commands (add, list, remove, inspect)
│   ├── doctor.go                   Dependency checker command
│   └── debug.go                    Debug utilities
├── config/config.go                Configuration loading
├── opencode.json                   OpenCode project config (model, permissions, plugins)
├── internal/
│   ├── opencode/opencode.go        OpenCode server lifecycle & REST client
│   ├── llm/
│   │   ├── provider.go             Two-stage Terraform generation pipeline
│   │   ├── prompts_aws.go          AWS-specific prompt content
│   │   └── prompts_azure.go        Azure-specific prompt content
│   ├── modules/
│   │   ├── types.go                ModuleMetadata, FitResult types
│   │   ├── store.go                SQLite CRUD for module metadata
│   │   ├── scanner.go              Git clone + HCL parsing (terraform-config-inspect)
│   │   ├── matcher.go              Fit score algorithm (coverage, specificity, var readiness)
│   │   └── prompt.go               User module prompt section builder
│   ├── provider/provider.go        Cloud provider detection (AWS, Azure)
│   ├── steampipe/
│   │   ├── client.go               Steampipe PostgreSQL client
│   │   └── resource_mapping.go     AWS ARN + Azure resource ID → table mapping
│   ├── terraform/
│   │   ├── generator.go            HCL file helpers
│   │   └── importer.go             terraform import runner
│   ├── tui/
│   │   ├── model.go                BubbleTea model & views (incl. module selection step)
│   │   ├── commands.go             Async tea.Cmd definitions
│   │   └── styles.go               Lipgloss styles
│   ├── cache/                      SQLite scan cache
│   ├── debuglog/                   File-based debug logger
│   ├── doctor/                     Dependency validation
│   └── graph/                      Resource dependency graph
├── .agents/skills/                 OpenCode Terraform skills (style guide, import, etc.)
├── docker/
│   └── entrypoint.sh               Docker container entrypoint
├── Dockerfile                       Multi-stage Docker build
├── Makefile                         Build & run targets
└── .sample.docker.env               Sample env file for Docker runs
```

## Keyboard Shortcuts (TUI)

| Key           | Action                                      |
|---------------|---------------------------------------------|
| `Enter`       | Select / confirm                            |
| `Space`       | Toggle resource or module selection         |
| `r`           | Expand related resources (resource step)    |
| `↑` / `↓`     | Navigate list / scroll code                 |
| `Esc`         | Go back to previous step                    |
| `/`           | Filter list                                 |
| `q` / `Ctrl+C`| Quit                                       |
