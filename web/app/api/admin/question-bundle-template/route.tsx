import { NextResponse } from "next/server";
import {
  QuestionBundleAnswerItemTemplate,
  QuestionBundleAnswerTemplate,
  QuestionBundleAudioTemplate,
  QuestionBundleDocumentTemplate,
  QuestionBundleImageTemplate,
  QuestionBundleOptionTemplate,
  QuestionBundleQuestionTemplate,
  QuestionBundleStatementTemplate,
  QuestionBundleTestTemplate,
} from "@/components/question-bundle/QuestionBundleDocument";

export const runtime = "nodejs";

export async function POST() {
  const ReactDOMServer = await import(/* webpackIgnore: true */ "react-dom/server");
  const render = ReactDOMServer.renderToStaticMarkup;
  return NextResponse.json({
    template: {
      document: `<!DOCTYPE html>\n${render(<QuestionBundleDocumentTemplate />)}`,
      test: render(<QuestionBundleTestTemplate />),
      question: render(<QuestionBundleQuestionTemplate />),
      option: render(<QuestionBundleOptionTemplate />),
      statement: render(<QuestionBundleStatementTemplate />),
      image: render(<QuestionBundleImageTemplate />),
      audio: render(<QuestionBundleAudioTemplate />),
      answer: render(<QuestionBundleAnswerTemplate />),
      answer_item: render(<QuestionBundleAnswerItemTemplate />),
    },
  });
}
