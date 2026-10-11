package permission

// PermissionServiceForTest is the concrete service, exported under a name
// that says what it is for: tests in OTHER packages that need to answer
// prompts on the real implementation (Subscribe, GrantFamily, ...) rather
// than on a stub. Production code uses the Service interface.
type PermissionServiceForTest = permissionService
