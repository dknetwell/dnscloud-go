package clients

import (
    "context"
    "time"
)

// Checker интерфейс для всех источников проверок
type Checker interface {
    Check(ctx context.Context, domain string) (*CheckResult, error)
    Name() string
    Timeout() time.Duration
    IsAvailable() bool
}

// CheckResult результат проверки
type CheckResult struct {
    Domain   string
    Category int
    TTL      int
    Metadata map[string]interface{}
}

// CheckerManager управляет несколькими источниками проверок
type CheckerManager struct {
    checkers []Checker
    enabled  map[string]bool
}

// NewCheckerManager создает менеджер проверок
func NewCheckerManager() *CheckerManager {
    return &CheckerManager{
        checkers: make([]Checker, 0),
        enabled:  make(map[string]bool),
    }
}

// AddChecker добавляет новый источник проверок
func (m *CheckerManager) AddChecker(checker Checker) {
    m.checkers = append(m.checkers, checker)
    m.enabled[checker.Name()] = true
}

// EnableChecker включает/выключает источник
func (m *CheckerManager) EnableChecker(name string, enabled bool) {
    m.enabled[name] = enabled
}

// CheckAll проверяет домен во всех включенных источниках
func (m *CheckerManager) CheckAll(ctx context.Context, domain string) map[string]*CheckResult {
    results := make(map[string]*CheckResult)
    
    for _, checker := range m.checkers {
        if !m.enabled[checker.Name()] {
            continue
        }
        
        // Запускаем каждую проверку с ее таймаутом
        checkerCtx, cancel := context.WithTimeout(ctx, checker.Timeout())
        defer cancel()
        
        if result, err := checker.Check(checkerCtx, domain); err == nil {
            results[checker.Name()] = result
        }
    }
    
    return results
}
