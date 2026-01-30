packer {
  required_plugins {
    azure = {
      version = "= 2.5.0"
      source  = "github.com/hashicorp/azure"
    }
  }
}
