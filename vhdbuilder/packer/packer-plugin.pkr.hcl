packer {
  required_plugins {
    azure = {
      version = ">= 2.0.1"
      source  = "github.com/hashicorp/azure"
    }
  }
}
