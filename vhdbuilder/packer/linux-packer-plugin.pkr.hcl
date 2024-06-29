packer {
  required_plugins {
    azure = {
      version = ">= 2.1.7"
      source  = "github.com/hashicorp/azure"
    }
  }
}