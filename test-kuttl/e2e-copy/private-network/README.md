# Private Network Test

This test validates that the operator is capable of running in private mode.

### Test Steps:

- 00. Create an IPFSCluster resource
- 05. Assert that an IPFSCluster StatefulSet was created.
- 10. Assert that one private node can view another's content.

### Still Needs:
- [ ] Test that public nodes aren't able to read private data