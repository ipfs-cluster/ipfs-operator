# IPFSCluster Test

This test validates that the operator is capable of creating the amount of required replicas,
and that data can be made available and accessed through IPFS.

### Test Steps:

- 00. Assert the Operator is running
- 05. Create an IPFSCluster resource
- 10. Assert the desired amount of replicas were created
- 15. Test creation on one replica and retrieval from another