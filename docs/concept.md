# Concept Model

This document defines the core concepts used by the workspace permission management system.

## Workspace

A workspace is a group of people created to achieve a specific goal. Workspaces are created by employees at manager level or above.

A workspace owns the permission configuration for the functions enabled inside it.

## Group

A group is a dynamic set of employees within a workspace. Workspace admins create groups by defining membership rules over employee attributes.

Example group rules include:

- Job title is `engineer`.
- Department is `ABCD-123`.
- Job title is `engineer` and department is `ABCD-123`.

Groups are used as permission subjects. Permissions grant actions to groups instead of directly granting actions to individual employees.

## Function

A function is an integrated capability provided by another system. Functions use the same ABAC-based permission mechanism as the workspace permission management system.

A workspace can enable multiple functions. Each function has a stable function key, such as `todo`, that identifies it in permission and resource records.

Each function defines the resource model it exposes to workspaces:

- Resource types: the kinds of resources managed by the function, such as `document`.
- Resource tags: labels assigned to resources for permission targeting, such as `section_1`.
- Resource actions: operations that can be performed on resources, such as `view`, `edit`, or `delete`.

## Resource

A resource is an object managed by a function inside a workspace. Resources belong to one workspace and one function.

Resources can have one or more resource tags. Permission rules use those tags to decide which groups can perform actions on the resource.

## Permission

A permission defines what a group can do to resources in a workspace.

Permissions are scoped by workspace and function. A permission grants one or more groups the ability to perform one or more resource actions on resources that have specific resource tags.

Example:

> Members of group `ABCD` can edit resources with the `section_1` tag.

## Relationship Summary

- A workspace contains multiple groups.
- A workspace can enable multiple functions.
- A function defines resource types, resource tags, and resource actions.
- A function resource belongs to one workspace and one function.
- A permission is configured in a workspace for a function.
- A permission grants groups actions over resources selected by resource tags.
