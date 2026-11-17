// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 fanti1

package main

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// Built-in translations. "auto" follows the OS language and falls back to English.

type language struct {
	code, name string
	text       map[string]string
}

var languages = []language{
	{"en", "English", map[string]string{
		"tagline": "Keeps CS2 off Core 0, automatically.", "open": "Open", "exit": "Exit", "status": "Status",
		"waiting": "Waiting for CS2...", "running_ok": "✔ CS2 without %s (PID %s)", "running_err": "✖ Failed on CS2 (PID %s)",
		"planned": "CS2 will use: CPU %s (of %s)", "verified": "Verified: CS2 uses CPU %s", "error": "Error: %s",
		"cpus_unknown": "System CPUs: not detected", "remove_group": "Remove from CS2", "core0": "Core 0", "core01": "Core 0 and 1",
		"remove0": "Remove Core 0", "remove01": "Remove Core 0 and 1", "autostart": "Start with system", "close_tray": "Close to tray",
		"language": "Language", "auto": "Automatic", "not_enough": "not enough CPUs", "rejected": "the system rejected the affinity",
		"tip_wait": "waiting for CS2", "tip_ok": "CS2 without %s", "tip_err": "ERROR: %s",
	}},
	{"pt", "Português", map[string]string{
		"tagline": "Tira o Core 0 do CS2, automaticamente.", "open": "Abrir", "exit": "Sair", "status": "Status",
		"waiting": "Aguardando CS2...", "running_ok": "✔ CS2 sem %s (PID %s)", "running_err": "✖ Falhou no CS2 (PID %s)",
		"planned": "CS2 vai usar: CPU %s (de %s)", "verified": "Verificado: CS2 usa CPU %s", "error": "Erro: %s",
		"cpus_unknown": "CPUs do sistema: não detectadas", "remove_group": "Remover do CS2", "core0": "Core 0", "core01": "Core 0 e 1",
		"remove0": "Remover Core 0", "remove01": "Remover Core 0 e 1", "autostart": "Iniciar com o sistema", "close_tray": "Fechar para a bandeja",
		"language": "Idioma", "auto": "Automático", "not_enough": "CPUs insuficientes", "rejected": "o sistema recusou a afinidade",
		"tip_wait": "aguardando CS2", "tip_ok": "CS2 sem %s", "tip_err": "ERRO: %s",
	}},
	{"es", "Español", map[string]string{
		"tagline": "Mantiene CS2 fuera del Core 0, automáticamente.", "open": "Abrir", "exit": "Salir", "status": "Estado",
		"waiting": "Esperando CS2...", "running_ok": "✔ CS2 sin %s (PID %s)", "running_err": "✖ Error en CS2 (PID %s)",
		"planned": "CS2 usará: CPU %s (de %s)", "verified": "Verificado: CS2 usa CPU %s", "error": "Error: %s",
		"cpus_unknown": "CPUs del sistema: no detectadas", "remove_group": "Quitar de CS2", "core0": "Core 0", "core01": "Core 0 y 1",
		"remove0": "Quitar Core 0", "remove01": "Quitar Core 0 y 1", "autostart": "Iniciar con el sistema", "close_tray": "Cerrar a la bandeja",
		"language": "Idioma", "auto": "Automático", "not_enough": "CPUs insuficientes", "rejected": "el sistema rechazó la afinidad",
		"tip_wait": "esperando CS2", "tip_ok": "CS2 sin %s", "tip_err": "ERROR: %s",
	}},
	{"ru", "Русский", map[string]string{
		"tagline": "Автоматически убирает Core 0 у CS2.", "open": "Открыть", "exit": "Выход", "status": "Статус",
		"waiting": "Ожидание CS2...", "running_ok": "✔ CS2 без %s (PID %s)", "running_err": "✖ Ошибка для CS2 (PID %s)",
		"planned": "CS2 будет использовать: CPU %s (из %s)", "verified": "Проверено: CS2 использует CPU %s", "error": "Ошибка: %s",
		"cpus_unknown": "CPU системы: не определены", "remove_group": "Убрать у CS2", "core0": "Core 0", "core01": "Core 0 и 1",
		"remove0": "Убрать Core 0", "remove01": "Убрать Core 0 и 1", "autostart": "Запускать вместе с системой", "close_tray": "Закрывать в трей",
		"language": "Язык", "auto": "Автоматически", "not_enough": "недостаточно CPU", "rejected": "система отклонила привязку",
		"tip_wait": "ожидание CS2", "tip_ok": "CS2 без %s", "tip_err": "ОШИБКА: %s",
	}},
	{"uk", "Українська", map[string]string{
		"tagline": "Автоматично прибирає Core 0 у CS2.", "open": "Відкрити", "exit": "Вихід", "status": "Статус",
		"waiting": "Очікування CS2...", "running_ok": "✔ CS2 без %s (PID %s)", "running_err": "✖ Помилка для CS2 (PID %s)",
		"planned": "CS2 використовуватиме: CPU %s (з %s)", "verified": "Перевірено: CS2 використовує CPU %s", "error": "Помилка: %s",
		"cpus_unknown": "CPU системи: не визначено", "remove_group": "Прибрати у CS2", "core0": "Core 0", "core01": "Core 0 і 1",
		"remove0": "Прибрати Core 0", "remove01": "Прибрати Core 0 і 1", "autostart": "Запускати разом із системою", "close_tray": "Закривати в трей",
		"language": "Мова", "auto": "Автоматично", "not_enough": "недостатньо CPU", "rejected": "система відхилила прив'язку",
		"tip_wait": "очікування CS2", "tip_ok": "CS2 без %s", "tip_err": "ПОМИЛКА: %s",
	}},
	{"de", "Deutsch", map[string]string{
		"tagline": "Hält CS2 automatisch von Core 0 fern.", "open": "Öffnen", "exit": "Beenden", "status": "Status",
		"waiting": "Warte auf CS2...", "running_ok": "✔ CS2 ohne %s (PID %s)", "running_err": "✖ Fehler bei CS2 (PID %s)",
		"planned": "CS2 nutzt: CPU %s (von %s)", "verified": "Geprüft: CS2 nutzt CPU %s", "error": "Fehler: %s",
		"cpus_unknown": "System-CPUs: nicht erkannt", "remove_group": "Von CS2 entfernen", "core0": "Core 0", "core01": "Core 0 und 1",
		"remove0": "Core 0 entfernen", "remove01": "Core 0 und 1 entfernen", "autostart": "Mit dem System starten", "close_tray": "In den Infobereich schließen",
		"language": "Sprache", "auto": "Automatisch", "not_enough": "nicht genug CPUs", "rejected": "das System hat die Zuordnung abgelehnt",
		"tip_wait": "warte auf CS2", "tip_ok": "CS2 ohne %s", "tip_err": "FEHLER: %s",
	}},
	{"fr", "Français", map[string]string{
		"tagline": "Éloigne CS2 du Core 0, automatiquement.", "open": "Ouvrir", "exit": "Quitter", "status": "État",
		"waiting": "En attente de CS2...", "running_ok": "✔ CS2 sans %s (PID %s)", "running_err": "✖ Échec sur CS2 (PID %s)",
		"planned": "CS2 utilisera : CPU %s (sur %s)", "verified": "Vérifié : CS2 utilise CPU %s", "error": "Erreur : %s",
		"cpus_unknown": "CPU du système : non détectés", "remove_group": "Retirer de CS2", "core0": "Core 0", "core01": "Core 0 et 1",
		"remove0": "Retirer le Core 0", "remove01": "Retirer les Core 0 et 1", "autostart": "Lancer au démarrage", "close_tray": "Fermer dans la zone de notification",
		"language": "Langue", "auto": "Automatique", "not_enough": "pas assez de CPU", "rejected": "le système a refusé l'affinité",
		"tip_wait": "en attente de CS2", "tip_ok": "CS2 sans %s", "tip_err": "ERREUR : %s",
	}},
	{"pl", "Polski", map[string]string{
		"tagline": "Automatycznie odsuwa CS2 od Core 0.", "open": "Otwórz", "exit": "Zakończ", "status": "Status",
		"waiting": "Czekam na CS2...", "running_ok": "✔ CS2 bez %s (PID %s)", "running_err": "✖ Błąd w CS2 (PID %s)",
		"planned": "CS2 użyje: CPU %s (z %s)", "verified": "Sprawdzono: CS2 używa CPU %s", "error": "Błąd: %s",
		"cpus_unknown": "Procesory systemu: nie wykryto", "remove_group": "Usuń z CS2", "core0": "Core 0", "core01": "Core 0 i 1",
		"remove0": "Usuń Core 0", "remove01": "Usuń Core 0 i 1", "autostart": "Uruchamiaj z systemem", "close_tray": "Zamykaj do zasobnika",
		"language": "Język", "auto": "Automatycznie", "not_enough": "za mało procesorów", "rejected": "system odrzucił koligację",
		"tip_wait": "czekam na CS2", "tip_ok": "CS2 bez %s", "tip_err": "BŁĄD: %s",
	}},
	{"tr", "Türkçe", map[string]string{
		"tagline": "CS2'yi otomatik olarak Core 0'dan uzak tutar.", "open": "Aç", "exit": "Çıkış", "status": "Durum",
		"waiting": "CS2 bekleniyor...", "running_ok": "✔ CS2, %s olmadan (PID %s)", "running_err": "✖ CS2'de hata (PID %s)",
		"planned": "CS2 kullanacak: CPU %s (%s içinden)", "verified": "Doğrulandı: CS2, CPU %s kullanıyor", "error": "Hata: %s",
		"cpus_unknown": "Sistem CPU'ları: algılanamadı", "remove_group": "CS2'den kaldır", "core0": "Core 0", "core01": "Core 0 ve 1",
		"remove0": "Core 0'ı kaldır", "remove01": "Core 0 ve 1'i kaldır", "autostart": "Sistemle başlat", "close_tray": "Kapatınca tepsiye küçült",
		"language": "Dil", "auto": "Otomatik", "not_enough": "yetersiz CPU", "rejected": "sistem benzeşimi reddetti",
		"tip_wait": "CS2 bekleniyor", "tip_ok": "CS2, %s olmadan", "tip_err": "HATA: %s",
	}},
	{"zh", "简体中文", map[string]string{
		"tagline": "自动让 CS2 避开 Core 0。", "open": "打开", "exit": "退出", "status": "状态",
		"waiting": "正在等待 CS2...", "running_ok": "✔ CS2 已移除 %s（PID %s）", "running_err": "✖ CS2 设置失败（PID %s）",
		"planned": "CS2 将使用：CPU %s（共 %s）", "verified": "已验证：CS2 使用 CPU %s", "error": "错误：%s",
		"cpus_unknown": "系统 CPU：未检测到", "remove_group": "从 CS2 移除", "core0": "Core 0", "core01": "Core 0 和 1",
		"remove0": "移除 Core 0", "remove01": "移除 Core 0 和 1", "autostart": "开机自启", "close_tray": "关闭时最小化到托盘",
		"language": "语言", "auto": "自动", "not_enough": "CPU 数量不足", "rejected": "系统拒绝了相关性设置",
		"tip_wait": "正在等待 CS2", "tip_ok": "CS2 已移除 %s", "tip_err": "错误：%s",
	}},
}

var (
	langSetting atomic.Value             // "auto" or a language code
	current     atomic.Pointer[language] // resolved language in use
)

// applyLanguage sets the language ("auto" or a code) and resolves it.
func applyLanguage(setting string) {
	langSetting.Store(setting)
	l := findLanguage(setting)
	if l == nil {
		l = findLanguage(detectLanguage())
	}
	current.Store(l)
}

func languageSetting() string {
	s, _ := langSetting.Load().(string)
	if s == "" {
		return "auto"
	}
	return s
}

func findLanguage(code string) *language {
	for i := range languages {
		if languages[i].code == code {
			return &languages[i]
		}
	}
	return nil
}

// detectLanguage matches the OS UI languages ("pt-BR", "pt_BR.UTF-8"...) to a built-in one.
func detectLanguage() string {
	for _, tag := range systemLanguages() {
		base, _, _ := strings.Cut(strings.ToLower(tag), "-")
		base, _, _ = strings.Cut(base, "_")
		if findLanguage(base) != nil {
			return base
		}
	}
	return "en"
}

// T returns the text for key in the current language, falling back to English.
func T(key string) string {
	if l := current.Load(); l != nil {
		if s, ok := l.text[key]; ok {
			return s
		}
	}
	return languages[0].text[key]
}

func Tf(key string, args ...any) string { return fmt.Sprintf(T(key), args...) }

// Texts for the window's legend, options and About page, merged into each language.
var moreText = map[string]map[string]string{
	"en": {"about": "About", "back": "Back", "version": "Version %s", "license": "License: GPL-3.0", "created_by": "Created by %s",
		"legend_used": "Used by CS2", "legend_removed": "Removed", "options": "Options",
		"about_text": "Removes Core 0 (or Cores 0 and 1) from CS2 automatically, keeping every other core. It never reads or writes game memory: it only uses the operating system's CPU affinity setting. No network, no telemetry."},
	"pt": {"about": "Sobre", "back": "Voltar", "version": "Versão %s", "license": "Licença: GPL-3.0", "created_by": "Criado por %s",
		"legend_used": "Usado pelo CS2", "legend_removed": "Removido", "options": "Opções",
		"about_text": "Remove o Core 0 (ou os Cores 0 e 1) do CS2 automaticamente, mantendo todos os outros núcleos. Nunca lê nem escreve a memória do jogo: usa apenas a afinidade de CPU do sistema operacional. Sem rede, sem telemetria."},
	"es": {"about": "Acerca de", "back": "Volver", "version": "Versión %s", "license": "Licencia: GPL-3.0", "created_by": "Creado por %s",
		"legend_used": "Usado por CS2", "legend_removed": "Quitado", "options": "Opciones",
		"about_text": "Quita el Core 0 (o los Cores 0 y 1) de CS2 automáticamente y mantiene todos los demás núcleos. Nunca lee ni escribe la memoria del juego: solo usa la afinidad de CPU del sistema operativo. Sin red ni telemetría."},
	"ru": {"about": "О программе", "back": "Назад", "version": "Версия %s", "license": "Лицензия: GPL-3.0", "created_by": "Автор: %s",
		"legend_used": "Использует CS2", "legend_removed": "Убрано", "options": "Настройки",
		"about_text": "Автоматически убирает Core 0 (или Core 0 и 1) у CS2, оставляя все остальные ядра. Никогда не читает и не изменяет память игры: использует только привязку к CPU операционной системы. Без сети и телеметрии."},
	"uk": {"about": "Про програму", "back": "Назад", "version": "Версія %s", "license": "Ліцензія: GPL-3.0", "created_by": "Автор: %s",
		"legend_used": "Використовує CS2", "legend_removed": "Прибрано", "options": "Налаштування",
		"about_text": "Автоматично прибирає Core 0 (або Core 0 і 1) у CS2, залишаючи всі інші ядра. Ніколи не читає і не змінює пам'ять гри: використовує лише прив'язку до CPU операційної системи. Без мережі й телеметрії."},
	"de": {"about": "Über", "back": "Zurück", "version": "Version %s", "license": "Lizenz: GPL-3.0", "created_by": "Erstellt von %s",
		"legend_used": "Von CS2 genutzt", "legend_removed": "Entfernt", "options": "Optionen",
		"about_text": "Entfernt Core 0 (oder Core 0 und 1) automatisch von CS2 und lässt alle anderen Kerne frei. Liest oder schreibt nie den Spielspeicher: nutzt nur die CPU-Zuordnung des Betriebssystems. Kein Netzwerk, keine Telemetrie."},
	"fr": {"about": "À propos", "back": "Retour", "version": "Version %s", "license": "Licence : GPL-3.0", "created_by": "Créé par %s",
		"legend_used": "Utilisé par CS2", "legend_removed": "Retiré", "options": "Options",
		"about_text": "Retire automatiquement le Core 0 (ou les Core 0 et 1) de CS2 en gardant tous les autres cœurs. Ne lit ni n'écrit jamais la mémoire du jeu : utilise uniquement l'affinité CPU du système. Pas de réseau, pas de télémétrie."},
	"pl": {"about": "O programie", "back": "Wstecz", "version": "Wersja %s", "license": "Licencja: GPL-3.0", "created_by": "Autor: %s",
		"legend_used": "Używany przez CS2", "legend_removed": "Usunięty", "options": "Opcje",
		"about_text": "Automatycznie usuwa Core 0 (lub Core 0 i 1) z CS2, zostawiając wszystkie pozostałe rdzenie. Nigdy nie czyta ani nie zapisuje pamięci gry: korzysta tylko z koligacji CPU systemu. Bez sieci i telemetrii."},
	"tr": {"about": "Hakkında", "back": "Geri", "version": "Sürüm %s", "license": "Lisans: GPL-3.0", "created_by": "Geliştiren: %s",
		"legend_used": "CS2 kullanıyor", "legend_removed": "Kaldırıldı", "options": "Seçenekler",
		"about_text": "Core 0'ı (veya Core 0 ve 1'i) CS2'den otomatik olarak kaldırır, diğer tüm çekirdekleri korur. Oyun belleğini asla okumaz veya yazmaz: yalnızca işletim sisteminin CPU benzeşimini kullanır. Ağ yok, telemetri yok."},
	"zh": {"about": "关于", "back": "返回", "version": "版本 %s", "license": "许可证：GPL-3.0", "created_by": "作者：%s",
		"legend_used": "CS2 使用", "legend_removed": "已移除", "options": "选项",
		"about_text": "自动从 CS2 移除 Core 0（或 Core 0 和 1），保留其他所有核心。从不读取或写入游戏内存：只使用操作系统的 CPU 相关性设置。无网络连接，无遥测。"},
}

func init() {
	for i := range languages {
		for k, v := range moreText[languages[i].code] {
			languages[i].text[k] = v
		}
	}
}
