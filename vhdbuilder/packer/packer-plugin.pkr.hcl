packer {
  required_plugins {
    azure = {
      version = "= 2.6.4"
      source  = "github.com/hashicorp/azure"
    }
  }
}
