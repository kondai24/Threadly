import { defineConfig } from "orval";

const inputPath = "../api/docs/swagger.yaml";
const apiOutputPath = "./src/orval";
const axiosPath = "./src/lib/apiClient.ts";
const axiosFunc = "customInstance";

export default defineConfig({
  api: {
    input: { target: inputPath },
    output: {
      mode: "split",
      target: apiOutputPath,
      client: "react-query",
      clean: true,
      override: {
        fetch: {
          includeHttpResponseReturnType: false,
        },
        mutator: {
          path: axiosPath,
          name: axiosFunc,
        },
        query: {
          useSuspenseQuery: true,
        },
      },
    },
    hooks: {
      afterAllFilesWrite: [`prettier --write ${apiOutputPath}`],
    },
  },
});
