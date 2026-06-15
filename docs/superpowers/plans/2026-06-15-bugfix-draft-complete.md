# Mengu Backend Bugfix + Draft Complete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all identified bugs (DocumentHandler, Gmail JWT, RBAC, worker logic) and complete the draft feature (Gmail Drafts API create + send on approve).

**Architecture:** Four phases executed sequentially. Each phase produces independently testable changes.

**Tech Stack:** Go 1.26, Gin, pgx/v5, Google Gmail API, Google Calendar API, golang-jwt

---
