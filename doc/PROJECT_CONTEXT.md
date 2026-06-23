# Hotel Chatbot Project

## Overview

WhatsApp Hotel Chatbot berbasis Golang yang menggunakan AI (Gemini/KoboLLM), SQLite, Knowledge Base lokal, Guest Profiling, Auto Learning, Ownership State Machine, dan Human Handover.

Bot digunakan untuk melayani pertanyaan tamu hotel secara otomatis dan dapat melakukan eskalasi ke Customer Service (Human Agent) jika diperlukan.

---

# Tech Stack

- Golang
- WhatsMeow
- SQLite
- Gemini AI
- KoboLLM
- WhatsApp Bot

---

# Core Features

## Guest Service

- FAQ Hotel
- Informasi Kamar
- Informasi Fasilitas
- Informasi Event
- Informasi Restaurant
- Reservasi
- Human Handover

## AI Features

- Multi Model Routing
- AI Template Matching
- Knowledge Base Search
- Auto Learning
- Guest Profiling
- VIP Detection
- Conversation Memory

## Data Features

- Guest Profile Database
- Chat History Database
- Knowledge Base Database
- Ownership State Machine

---

# Current Status

## Completed

### H1 - Memory Cleanup Fix

Status: ✅ DONE

### H2 - VIP Logic Fix

Status: ✅ DONE

### H3 - Query Construction Fix

Status: ✅ DONE

### H4 - Seed Data Fix

Status: ✅ DONE

### H5 - RoomURLMap

Status: ✅ VERIFIED CORRECT
Tidak memerlukan perubahan.

### Graceful Shutdown

Status: ✅ DONE

Implemented:

- SIGINT Handler
- SIGTERM Handler
- appCtx / appCancel
- activeWorkers WaitGroup
- Atomic Shutdown Flag
- Database Safe Close
- WhatsApp Safe Disconnect
- Background Goroutine Shutdown

Verified:

- Tidak ada database is closed
- Tidak ada database is locked
- Tidak ada panic saat shutdown

### Config Manager

Status: ✅ DONE

Migrated To Config:

- Gemini API Key
- AI Model Routing
- RoomURLMap
- Kontak Reservasi
- HumanResponseTimeout

Notes:

Masih terdapat beberapa internal constants yang dapat dipindahkan ke config di masa depan, namun tidak dianggap blocker.

---

# Important Files

- main.go
- knowledge.go
- learning.go
- seed_knowledge.go
- config.go
- .env

---

# Database

## hotel_knowledge.db

Knowledge Base Hotel

## guest_profiles.db

Guest Profile
Chat History
Visit Tracking
VIP Tracking

## whatsapp_session.db

WhatsApp Session Storage

---

# Architecture Rules

- Jangan ubah business logic tanpa alasan yang jelas.
- Jangan redesign arsitektur besar.
- Jangan pecah package sebelum fase refactor.
- Pertahankan kompatibilitas fitur existing.
- Stabilitas lebih penting daripada fitur baru.

---

# Current Roadmap

## Phase 4A ✅ DONE

Retry AI Call

Target:

- Retry 429
- Retry 503
- Retry Timeout
- Exponential Backoff

## Phase 4B ✅ DONE

TTL Cleanup

Target:

- conversationOwners TTL
- Session TTL
- Memory Cleanup Improvement

## Phase 5

Refactor main.go

Target:

- Kurangi monolith
- Pisahkan responsibility
- Tingkatkan maintainability

## Phase 6

Admin API

Target:

- Endpoint internal
- Monitoring
- Statistics

## Phase 7

Web Panel

Target:

- User Management
- Role Management
- Config Management
- Knowledge Base Management

## Phase 8

CRM

Target:

- Guest Tracking
- Follow Up
- Lead Management

## Phase 9

AI Analytics

Target:

- Top Questions
- Complaint Analysis
- Room Interest Analysis

## Phase 10

AI Command Center

Target:

- Natural Language Query
- Business Insights
- Weekly Reports
- Monthly Reports

---

# Long Term Goals

- Multi Hotel
- Multi Property
- Multi User
- Multi Role
- SaaS Architecture
- CRM Integration
- AI Reporting
- AI Business Intelligence
