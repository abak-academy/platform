import styles from "./ExamCardPrintable.module.css";

export interface ExamCardData {
  fullName: string;
  participantNumber: string;
  school: string;
  grade: string;
  dob: string;
  photoUrl?: string;
  examName: string;
  subject: string;
  date: string;
  timeRange: string;
  duration: string;
  mode: string;
  platform: string;
  checkInCode: string;
}

const stroke = {
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 2,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
};

const Icons = {
  user: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <circle cx="12" cy="8" r="4" />
      <path d="M4 22c0-4.4 3.6-8 8-8s8 3.6 8 8" />
    </svg>
  ),
  id: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <rect x="2" y="5" width="20" height="14" rx="2" />
      <path d="M7 15h4M15 10h3M15 14h2" />
      <circle cx="8.5" cy="10.5" r="1.6" />
    </svg>
  ),
  school: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <path d="M3 21V9l9-6 9 6v12" />
      <path d="M9 21v-6h6v6" />
    </svg>
  ),
  grade: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <path d="M4 19.5V6a2 2 0 0 1 2-2h13v16H6a2 2 0 0 0-2 2Z" />
      <path d="M9 8h7" />
    </svg>
  ),
  calendar: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <rect x="3" y="4" width="18" height="18" rx="2" />
      <path d="M3 10h18M8 2v4M16 2v4" />
    </svg>
  ),
  book: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <path d="M4 5a2 2 0 0 1 2-2h9v18H6a2 2 0 0 0-2 2z" />
      <path d="M15 3h3a1 1 0 0 1 1 1v15" />
    </svg>
  ),
  folder: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <path d="M4 4h13l3 3v13H4z" />
      <path d="M4 9h12" />
    </svg>
  ),
  clock: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5l3 2" />
    </svg>
  ),
  timer: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <path d="M12 2v3M9 2h6" />
      <circle cx="12" cy="14" r="8" />
      <path d="M12 14V9" />
    </svg>
  ),
  monitor: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <rect x="2" y="4" width="20" height="13" rx="2" />
      <path d="M8 21h8M12 17v4" />
    </svg>
  ),
  pin: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <path d="M12 21s7-5.5 7-11a7 7 0 1 0-14 0c0 5.5 7 11 7 11Z" />
      <circle cx="12" cy="10" r="2.5" />
    </svg>
  ),
  clipboard: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <rect x="5" y="3" width="14" height="18" rx="2" />
      <path d="M9 3h6v3H9zM8 11h8M8 15h6" />
    </svg>
  ),
  cap: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <path d="M22 10 12 5 2 10l10 5 10-5Z" />
      <path d="M6 12v5c0 1 3 3 6 3s6-2 6-3v-5" />
    </svg>
  ),
  idcard: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <rect x="2" y="5" width="20" height="14" rx="2" />
      <circle cx="8" cy="11" r="2.4" />
      <path d="M14 10h4M14 14h4M5 16c.6-1.6 1.7-2.4 3-2.4s2.4.8 3 2.4" />
    </svg>
  ),
  warn: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 8v5M12 16.5h.01" />
    </svg>
  ),
  phone: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <path d="M4 12a8 8 0 0 1 16 0" />
      <rect x="2.5" y="12" width="4" height="6" rx="2" />
      <rect x="17.5" y="12" width="4" height="6" rx="2" />
      <path d="M20 18v1a3 3 0 0 1-3 3h-3" />
    </svg>
  ),
  globe: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18M12 3c3 3 3 15 0 18M12 3c-3 3-3 15 0 18" />
    </svg>
  ),
  instagram: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <rect x="3" y="3" width="18" height="18" rx="5" />
      <circle cx="12" cy="12" r="4" />
      <circle cx="17.5" cy="6.5" r="1" />
    </svg>
  ),
  youtube: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <rect x="2.5" y="6" width="19" height="12" rx="4" />
      <path d="M10 9.5v5l4-2.5-4-2.5Z" fill="currentColor" />
    </svg>
  ),
  x: (
    <svg viewBox="0 0 24 24" {...stroke}>
      <path d="M4 4l16 16M20 4L4 20" />
    </svg>
  ),
};

// Decorative barcode (deterministic, not a scannable symbology).
// TODO: uncertain — swap for a real barcode (e.g. jsbarcode Code128) once the
// participant-number source is finalized, verify scannability before prod.
function BarcodeBars({ value }: { value: string }) {
  const seed = value || "000000";
  const bars: { x: number; w: number }[] = [];
  let x = 0;
  for (let i = 0; i < 58; i++) {
    const c = seed.charCodeAt(i % seed.length) || 42;
    const bw = 1.4 + ((c + i * 7) % 5) * 0.9;
    const gap = 1.2 + ((c * 3 + i) % 3) * 0.7;
    if ((c + i) % 3 !== 0) bars.push({ x, w: bw });
    x += bw + gap;
  }
  return (
    <svg viewBox={`0 0 ${x.toFixed(1)} 60`} preserveAspectRatio="none" aria-hidden="true">
      {bars.map((b, i) => (
        <rect key={i} x={b.x.toFixed(1)} y="0" width={b.w.toFixed(1)} height="60" fill="#22315b" />
      ))}
    </svg>
  );
}

const AbakMarkFull = (
  <svg viewBox="0 0 120 120" fill="none" aria-label="abak academy">
    <circle cx="44" cy="34" r="15" fill="#22315B" />
    <path d="M22 104 Q22 64 44 64 Q66 64 66 104 Z" fill="#22315B" />
    <path d="M62 104 Q62 78 80 78 Q98 78 98 104 Z" fill="#1E978A" />
    <path d="M80 44 L96 51 L80 58 L64 51 Z" fill="#D99A2B" />
    <circle cx="80" cy="62" r="11" fill="#1E978A" />
    <rect x="79" y="44" width="2.5" height="9" fill="#D99A2B" />
  </svg>
);

const PhotoPlaceholder = (
  <svg viewBox="0 0 120 120" preserveAspectRatio="xMidYMid slice">
    <rect width="120" height="120" fill="#DCE9FF" />
    <circle cx="60" cy="50" r="26" fill="#B9C9EC" />
    <path d="M18 120c0-25 19-38 42-38s42 13 42 38H18Z" fill="#9FB6E4" />
    <path d="M44 98c3 7 9 11 16 11s13-4 16-11v-8H44v8Z" fill="#fff" />
    <path d="M60 96l-8 5 8 21 8-21-8-5Z" fill="#22315B" />
  </svg>
);

function Field({
  icon,
  label,
  value,
  isName = false,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  isName?: boolean;
}) {
  return (
    <>
      <span className={styles.ic}>{icon}</span>
      <span className={styles.k}>{label}</span>
      <span className={styles.c}>:</span>
      <span className={`${styles.v} ${isName ? styles.name : ""}`} title={value}>
        {value}
      </span>
    </>
  );
}

function DetailRow({
  icon,
  label,
  children,
}: {
  icon: React.ReactNode;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className={styles.drow}>
      <span className={styles.ic}>{icon}</span>
      <span className={styles.k}>{label}</span>
      <span className={styles.c}>:</span>
      <span className={styles.v}>{children}</span>
    </div>
  );
}

export function ExamCardPrintable(props: ExamCardData) {
  return (
    <div className={styles.card}>
      {/* Header */}
      <header className={styles.head}>
        <div className={styles.banner}>
          <div className={styles.sliver} />
          <p>
            SUKSES UJIAN HARI INI,
            <br />
            CERDAS MASA DEPANMU
          </p>
          <div className={styles.seal}>{Icons.cap}</div>
        </div>
        <div className={styles.brand}>
          <div className={styles.mark}>{AbakMarkFull}</div>
          <div className={styles.titles}>
            <h1>KARTU PESERTA UJIAN</h1>
            <div className={styles.sub}>Abak Academy · Exam Participant Card</div>
            <div className={styles.note}>Bawa kartu ini saat mengikuti ujian</div>
          </div>
        </div>
      </header>

      {/* Body */}
      <section className={styles.body}>
        <div className={styles.left}>
          <div className={`${styles.panel} ${styles.peserta}`}>
            <div className={styles.photo}>
              {props.photoUrl ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={props.photoUrl} alt={props.fullName} />
              ) : (
                PhotoPlaceholder
              )}
            </div>
            <div className={styles.fields}>
              <Field icon={Icons.user} label="Nama Lengkap" value={props.fullName} isName />
              <Field icon={Icons.id} label="Nomor Peserta" value={props.participantNumber} />
              <Field icon={Icons.school} label="Asal Sekolah" value={props.school} />
              <Field icon={Icons.grade} label="Kelas" value={props.grade} />
              <Field icon={Icons.calendar} label="Tanggal Lahir" value={props.dob} />
            </div>
          </div>

          <div className={`${styles.panel} ${styles.idcard}`}>
            <div className={styles.idrow}>
              <div className={styles.badge}>{Icons.idcard}</div>
              <div className={styles.idgroup}>
                <div className={styles.idcol}>
                  <div className={styles.k}>Nomor Peserta</div>
                  <div className={styles.num}>{props.participantNumber}</div>
                </div>
                <div className={styles.idsep} />
                <div className={styles.idcol}>
                  <div className={styles.k}>Kode Check-in</div>
                  <div className={styles.pill}>{props.checkInCode}</div>
                </div>
              </div>
            </div>
            <div className={styles.barcode}>
              <BarcodeBars value={props.checkInCode} />
            </div>
          </div>
        </div>

        <div className={`${styles.panel} ${styles.detail}`}>
          <div className={styles.cap}>
            {Icons.clipboard}
            Detail Ujian
          </div>
          <div className={styles.rows}>
            <DetailRow icon={Icons.book} label="Nama Ujian">
              {props.examName}
            </DetailRow>
            <DetailRow icon={Icons.folder} label="Paket / Mapel">
              {props.subject}
            </DetailRow>
            <DetailRow icon={Icons.calendar} label="Tanggal">
              {props.date}
            </DetailRow>
            <DetailRow icon={Icons.clock} label="Waktu">
              {props.timeRange}
            </DetailRow>
            <DetailRow icon={Icons.timer} label="Durasi">
              {props.duration}
            </DetailRow>
            <DetailRow icon={Icons.monitor} label="Mode Ujian">
              {props.mode}
            </DetailRow>
            <DetailRow icon={Icons.pin} label="Platform">
              {props.platform}
            </DetailRow>
          </div>
        </div>
      </section>

      {/* Perhatian */}
      <section className={styles.notice}>
        <div className={styles.noticeMark}>
          <span className={styles.dot}>{Icons.warn}</span>
          <b>Perhatian</b>
        </div>
        <ul>
          <li>Datang 30 menit sebelum ujian dimulai.</li>
          <li>Dilarang membuka tab atau aplikasi lain saat ujian.</li>
          <li>Siapkan perangkat, koneksi internet stabil, dan kartu ini.</li>
          <li>Pelanggaran dapat berakibat diskualifikasi.</li>
        </ul>
      </section>

      {/* Footer */}
      <footer className={styles.foot}>
        <div className={styles.fitem}>
          <span className={styles.fic}>{Icons.phone}</span>
          <div className={styles.ft}>
            <div className={styles.t1}>Butuh Bantuan?</div>
            <div className={styles.t2}>0812-3456-7890</div>
          </div>
        </div>
        <span className={styles.sep} />
        <div className={styles.fitem}>
          <span className={styles.fic}>{Icons.globe}</span>
          <div className={styles.ft}>
            <div className={styles.t1}>Pusat Bantuan</div>
            <div className={styles.t2}>help.abakacademy.id</div>
          </div>
        </div>
        <div className={styles.socials}>
          <span className={styles.s}>{Icons.instagram}</span>
          <span className={styles.s}>{Icons.youtube}</span>
          <span className={styles.s}>{Icons.x}</span>
          <span className={styles.handle}>@abakacademy</span>
        </div>
      </footer>
    </div>
  );
}
