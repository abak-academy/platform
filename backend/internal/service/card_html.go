package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
)

// Exam card geometry (FR-20..24): A6 landscape, 148 x 105 mm, mm 1:1, top-left
// origin — the same page mirrored from the retired PDF renderer, now laid
// out with CSS instead of manual coordinate math.
const (
	cardPageWidthMm  = 148.0
	cardPageHeightMm = 105.0

	cardNavyHex        = "#22315B"
	cardGoldHex        = "#D99A2B"
	cardGoldBgHex      = "#FBEFCF"
	cardTealHex        = "#1E978A"
	cardTealDarkHex    = "#137063"
	cardTealTintHex    = "#E6F4F1"
	cardInkHex         = "#2B3648"
	cardPlaceholderHex = "#B7BECE"
)

var cardHTMLTemplate = template.Must(template.New("card").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>{{.StyleBlock}}</style>
</head>
<body>
<div class="card-backdrop"></div>
<div class="card-panel"></div>
<div class="card-header">
{{if .HasLogo}}<div class="card-logo-frame"><img class="card-logo" src="data:{{.LogoMime}};base64,{{.LogoBase64}}" alt=""></div>{{end}}
<div class="card-header-title">KARTU PESERTA UJIAN</div>
{{if .TenantName}}<div class="card-header-tenant">{{.TenantName}}</div>{{end}}
</div>
<div class="card-photo-mat"></div>
<div class="card-photo-frame">
{{if .HasPhoto}}<img class="card-photo" src="data:{{.PhotoMime}};base64,{{.PhotoBase64}}" alt="">
{{else if .Initials}}<div class="card-photo-initials">{{.Initials}}</div>
{{else}}<div class="card-photo-placeholder"><div class="card-photo-placeholder-head"></div><div class="card-photo-placeholder-body"></div></div>
{{end}}
</div>
<div class="card-detail">
<div class="card-label" style="top:0mm">NAMA</div>
<div class="card-name">{{.StudentName}}</div>
<div class="card-label" style="top:10mm">UJIAN</div>
<div class="card-exam-title">{{.ExamTitle}}</div>
<div class="card-label" style="top:22.5mm">JADWAL</div>
<div class="card-schedule">{{.ScheduleText}}</div>
</div>
<div class="card-token-band">
<div class="card-token-tear"></div>
<div class="card-token-notch card-token-notch-left"></div>
<div class="card-token-notch card-token-notch-right"></div>
<div class="card-token-label">TOKEN AKSES</div>
<div class="card-token-value">{{.Token}}</div>
</div>
<div class="card-footer">{{.FooterNote}}</div>
<script>{{.Script}}</script>
</body>
</html>`))

// cardShrinkScript shrinks the token text if it overflows its band (FR-22,
// Invariant 5: the token string itself is never truncated), mirroring
// certificateFitScript's approach for HTML/CSS rendering.
const cardShrinkScript = template.JS(`(function(){
var minPt = 8;
var margin = 0.95;
var el = document.querySelector('.card-token-value');
if (!el) return;
var size = parseFloat(getComputedStyle(el).fontSize) || 19;
while (el.scrollWidth > el.clientWidth && size > minPt) {
  size = Math.max(minPt, size * margin);
  el.style.fontSize = size + 'pt';
}
})();`)

type cardHTMLData struct {
	StyleBlock   template.CSS
	HasLogo      bool
	LogoMime     string
	LogoBase64   string
	TenantName   string
	HasPhoto     bool
	PhotoMime    string
	PhotoBase64  string
	Initials     string
	StudentName  string
	ExamTitle    string
	ScheduleText string
	Token        string
	FooterNote   string
	Script       template.JS
}

// buildCardHTML renders reg/studentName/tenantName/logoImg/photoImg into
// self-contained exam card HTML (FR-20..24): no DB/network access, no
// PDF library. logoImg/photoImg are already-fetched image bytes (or nil) —
// fetching app_logo_url/User.PhotoURL is I/O that belongs at the call site
// (Service.GetExamCard), so a network failure there never fails card
// generation (FR-21): a missing/unfetchable/corrupt logo just omits the
// mark, and a missing/corrupt photo falls back to an initials avatar (or a
// neutral placeholder when no name is available either).
func buildCardHTML(reg *model.RegistrationDetail, studentName, tenantName string, logoImg, photoImg []byte) ([]byte, error) {
	faces, err := certificateFontFaces()
	if err != nil {
		return nil, fmt.Errorf("build card font faces: %w", err)
	}

	name := studentName
	if name == "" {
		name = "-"
	}

	data := cardHTMLData{
		StyleBlock:   buildCardStyleBlock(faces),
		TenantName:   tenantName,
		StudentName:  name,
		ExamTitle:    reg.Exam.Title,
		ScheduleText: cardScheduleText(reg),
		Token:        reg.Token,
		FooterNote:   cardFooterNote(reg),
		Script:       cardShrinkScript,
	}

	if mime, ok := decodeImageMime(logoImg); ok {
		data.HasLogo = true
		data.LogoMime = mime
		data.LogoBase64 = base64.StdEncoding.EncodeToString(logoImg)
	}
	if mime, ok := decodeImageMime(photoImg); ok {
		data.HasPhoto = true
		data.PhotoMime = mime
		data.PhotoBase64 = base64.StdEncoding.EncodeToString(photoImg)
	} else {
		data.Initials = nameInitials(studentName)
	}

	var buf bytes.Buffer
	if err := cardHTMLTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute card html template: %w", err)
	}
	return buf.Bytes(), nil
}

// decodeImageMime validates image bytes and returns the data-URI mime
// type to embed them with, never failing on missing/corrupt input (FR-21) —
// callers treat ok=false as "omit this image". Shared with the certificate
// renderer, whose backgrounds and signatures are uploaded as any image type.
func decodeImageMime(data []byte) (mime string, ok bool) {
	if len(data) == 0 {
		return "", false
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width == 0 || cfg.Height == 0 {
		return "", false
	}
	switch format {
	case "png":
		return "image/png", true
	case "jpeg":
		return "image/jpeg", true
	case "gif":
		return "image/gif", true
	default:
		return "", false
	}
}

// nameInitials returns up to two uppercase initials from a name: the first
// letter of the first and last words, or just the first letter for one word.
func nameInitials(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return ""
	}
	first := []rune(fields[0])
	out := strings.ToUpper(string(first[0]))
	if len(fields) > 1 {
		last := []rune(fields[len(fields)-1])
		out += strings.ToUpper(string(last[0]))
	}
	return out
}

// cardScheduleText preserves the pre-existing schedule formatting: Asia/Jakarta,
// "02 Jan 2006 15:04 WIB" (FR-23).
func cardScheduleText(reg *model.RegistrationDetail) string {
	if reg.Exam.ScheduledAt == nil {
		return "-"
	}
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.UTC
	}
	return reg.Exam.ScheduledAt.In(loc).Format("02 Jan 2006 15:04 WIB")
}

// cardFooterNote preserves the pre-existing check-in vs free-access copy,
// keyed on reg.Exam.RequiresCheckin / CheckInWindowMinutes.
func cardFooterNote(reg *model.RegistrationDetail) string {
	if reg.Exam.RequiresCheckin {
		if reg.Exam.CheckInWindowMinutes != nil {
			return fmt.Sprintf("Harap check-in dalam waktu %d menit sebelum ujian.", *reg.Exam.CheckInWindowMinutes)
		}
		return "Harap check-in sebelum ujian dimulai."
	}
	return "Akses bebas pada waktu yang ditentukan."
}

// buildCardStyleBlock emits the @page rule, every @font-face (reusing the
// certificate builder's font faces), and the fixed card layout as a single
// trusted CSS blob — the card has no admin-configurable design, so unlike
// the certificate builder this needs no per-field style computation.
func buildCardStyleBlock(faces []certificateFontFace) template.CSS {
	var b bytes.Buffer
	fmt.Fprintf(&b, "@page{size:%.0fmm %.0fmm;margin:0;}", cardPageWidthMm, cardPageHeightMm)
	b.WriteString("*{margin:0;padding:0;box-sizing:border-box;}")
	fmt.Fprintf(&b, "html,body{width:%.0fmm;height:%.0fmm;font-family:%s;position:relative;overflow:hidden;}", cardPageWidthMm, cardPageHeightMm, FontPublicSans)
	for _, face := range faces {
		fmt.Fprintf(&b, "@font-face{font-family:%s;font-weight:%s;src:url(data:font/ttf;base64,%s) format(\"truetype\");}", face.Family, face.Weight, face.Base64)
	}

	fmt.Fprintf(&b, ".card-backdrop{position:absolute;left:0;top:18mm;width:%.0fmm;height:%.0fmm;background:%s;}", cardPageWidthMm, cardPageHeightMm-18, cardTealTintHex)
	fmt.Fprintf(&b, ".card-panel{position:absolute;left:6mm;top:22mm;width:%.0fmm;height:58mm;border-radius:3mm;background:#fff;}", cardPageWidthMm-12)

	fmt.Fprintf(&b, ".card-header{position:absolute;left:0;top:0;width:%.0fmm;height:18mm;background:linear-gradient(90deg,%s,%s);color:#fff;}", cardPageWidthMm, cardNavyHex, cardTealHex)
	b.WriteString(".card-logo-frame{position:absolute;left:5.5mm;top:3.5mm;width:11mm;height:11mm;border-radius:2mm;background:#fff;display:flex;align-items:center;justify-content:center;}")
	b.WriteString(".card-logo{width:9mm;height:9mm;object-fit:contain;}")
	fmt.Fprintf(&b, ".card-header-title{position:absolute;left:20mm;top:3.5mm;width:%.0fmm;font-family:%s;font-weight:bold;font-size:12pt;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;}", cardPageWidthMm-24, FontSourceSerif4)
	fmt.Fprintf(&b, ".card-header-tenant{position:absolute;left:20mm;top:10.5mm;width:%.0fmm;color:%s;font-size:7pt;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;}", cardPageWidthMm-24, cardGoldHex)

	b.WriteString(".card-photo-mat{position:absolute;left:6.8mm;top:22.8mm;width:24.4mm;height:30.4mm;border-radius:2mm;background:" + cardTealHex + ";}")
	b.WriteString(".card-photo-frame{position:absolute;left:8mm;top:24mm;width:22mm;height:28mm;background:#fff;overflow:hidden;display:flex;align-items:center;justify-content:center;}")
	b.WriteString(".card-photo{width:100%;height:100%;object-fit:cover;}")
	b.WriteString(".card-photo-initials{width:100%;height:100%;background:" + cardTealTintHex + ";color:" + cardNavyHex + ";font-family:" + FontSourceSerif4 + ";font-weight:bold;font-size:20pt;display:flex;align-items:center;justify-content:center;}")
	b.WriteString(".card-photo-placeholder{position:relative;width:100%;height:100%;background:" + cardTealTintHex + ";overflow:hidden;}")
	b.WriteString(".card-photo-placeholder-head{position:absolute;left:50%;top:22%;width:34%;height:26%;transform:translate(-50%,-50%);border-radius:50%;background:" + cardPlaceholderHex + ";}")
	b.WriteString(".card-photo-placeholder-body{position:absolute;left:50%;top:98%;width:80%;height:60%;transform:translate(-50%,-50%);border-radius:50%;background:" + cardPlaceholderHex + ";}")

	fmt.Fprintf(&b, ".card-detail{position:absolute;left:36mm;top:24mm;width:%.0fmm;}", 142.0-36.0)
	b.WriteString(".card-label{position:absolute;color:" + cardTealDarkHex + ";font-weight:bold;font-size:6pt;}")
	b.WriteString(".card-name{position:absolute;top:3.4mm;width:100%;color:" + cardNavyHex + ";font-family:" + FontSourceSerif4 + ";font-weight:bold;font-size:10pt;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;}")
	b.WriteString(".card-exam-title{position:absolute;top:13.4mm;width:100%;color:" + cardInkHex + ";font-size:9pt;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;text-overflow:ellipsis;line-height:1.3;}")
	b.WriteString(".card-schedule{position:absolute;top:25.9mm;width:100%;color:" + cardInkHex + ";font-size:8pt;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;}")

	b.WriteString(".card-token-band{position:absolute;left:8mm;top:64mm;width:132mm;height:14mm;border-radius:2.2mm;background:" + cardGoldBgHex + ";}")
	b.WriteString(".card-token-tear{position:absolute;left:5mm;right:5mm;top:4.6mm;border-top:0.5pt dashed " + cardGoldHex + ";}")
	b.WriteString(".card-token-notch{position:absolute;top:2.7mm;width:3.2mm;height:3.2mm;border-radius:50%;background:" + cardTealTintHex + ";}")
	b.WriteString(".card-token-notch-left{left:-1.6mm;}")
	b.WriteString(".card-token-notch-right{right:-1.6mm;}")
	b.WriteString(".card-token-label{position:absolute;left:0;top:1.1mm;width:100%;text-align:center;color:" + cardGoldHex + ";font-weight:bold;font-size:5.5pt;}")
	b.WriteString(".card-token-value{position:absolute;left:7mm;right:7mm;top:6mm;text-align:center;color:" + cardNavyHex + ";font-family:" + FontSourceSerif4 + ";font-weight:bold;font-size:19pt;white-space:nowrap;}")

	b.WriteString(".card-footer{position:absolute;left:8mm;top:82mm;width:132mm;color:" + cardNavyHex + ";font-size:6.5pt;line-height:1.4;}")

	return template.CSS(b.String())
}
