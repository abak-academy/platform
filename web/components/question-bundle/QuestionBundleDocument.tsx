const styles = `
@page { size: A4; margin: 16mm 14mm; }
body { margin: 0; color: #111827; font-family: Inter, Arial, sans-serif; font-size: 12px; line-height: 1.45; }
.bundle-cover { display: flex; min-height: 240mm; page-break-after: always; flex-direction: column; justify-content: center; text-align: center; }
.bundle-cover h1 { margin: 10px 0 4px; font-size: 30px; }
.bundle-brand { color: #4f46e5; font-size: 13px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
.bundle-test { page-break-before: always; }
.bundle-test-header { margin-bottom: 18px; border-bottom: 2px solid #111827; padding-bottom: 8px; }
.bundle-muted { color: #6b7280; }
.bundle-question { margin: 0 0 18px; break-inside: avoid; }
.bundle-question-body { margin-bottom: 8px; }
.bundle-options, .bundle-statements { margin: 8px 0 0; padding-left: 22px; }
.bundle-media { margin: 8px 0; max-width: 100%; }
.bundle-audio { display: inline-block; margin: 6px 0; border-radius: 999px; background: #fff7ed; padding: 4px 8px; color: #9a3412; font-weight: 700; }
.bundle-answer { margin-top: 8px; border-left: 3px solid #b91c1c; background: #fef2f2; padding: 8px 10px; }
.bundle-answer strong { color: #b91c1c; letter-spacing: .04em; }
`;

export function QuestionBundleDocumentTemplate() {
  return (
    <html>
      <head>
        <meta charSet="utf-8" />
        <style>{styles}</style>
      </head>
      <body>
        <section className="bundle-cover">
          <div className="bundle-brand">Abak Academy</div>
          <h1>{"{{bundle_title}}"}</h1>
          <p className="bundle-muted">{"{{bundle_variant_label}}"}</p>
        </section>
        {"{{tests_html}}"}
      </body>
    </html>
  );
}

export function QuestionBundleTestTemplate() {
  return (
    <section className="bundle-test">
      <header className="bundle-test-header">
        <h2>{"{{test_title}}"}</h2>
        <p className="bundle-muted">{"{{test_meta}}"}</p>
      </header>
      {"{{questions_html}}"}
    </section>
  );
}

export function QuestionBundleQuestionTemplate() {
  return (
    <article className="bundle-question">
      <h3>Soal {"{{question_number}}"} <span className="bundle-muted">({"{{question_format}}"})</span></h3>
      <div className="bundle-question-body">{"{{question_body_html}}"}</div>
      <ol className="bundle-statements">{"{{statements_html}}"}</ol>
      {"{{question_image_html}}"}
      {"{{audio_html}}"}
      <ol className="bundle-options" type="A">{"{{options_html}}"}</ol>
      {"{{answer_html}}"}
    </article>
  );
}

export function QuestionBundleOptionTemplate() {
  return <li><strong>{"{{option_key}}"}.</strong> {"{{option_text}}"}{"{{option_image_html}}"}</li>;
}

export function QuestionBundleStatementTemplate() {
  return <li>{"{{statement_text}}"}</li>;
}

export function QuestionBundleImageTemplate() {
  return <img className="bundle-media" src="{{image_url}}" alt="{{image_alt}}" />;
}

export function QuestionBundleAudioTemplate() {
  return <p className="bundle-audio">Soal ini memiliki audio — diputar oleh pengawas</p>;
}

export function QuestionBundleAnswerTemplate() {
  return <aside className="bundle-answer"><strong>KUNCI JAWABAN — JANGAN DIBAGIKAN</strong>{"{{answer_items_html}}"}</aside>;
}

export function QuestionBundleAnswerItemTemplate() {
  return <p>{"{{answer_item}}"}</p>;
}
