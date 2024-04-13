packer {
  required_plugins {
    azure = {
      version = ">= 2.0.4"
      source  = "github.com/hashicorp/azure"
    }
  }
}
