locals {
  # Use the generated VMID from the random_integer resource
  vmid = random_integer.next_vmid.result
}