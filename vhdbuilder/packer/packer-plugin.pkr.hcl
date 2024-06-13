packer {
  required_plugins {
    azure = {
      version = ">= 2.1.6"
      source  = "github.com/hashicorp/azure"
    }
  }
}
