import { Typography, Button } from "antd";
import { GithubIcon } from "./icons";

function Footer() {
  const { Text } = Typography;
  return (
    <div className="app-footer">
      <Text type="secondary">© 2026~ ming-kang</Text>
      <Button
        onClick={() =>
          window.open(
            "https://github.com/ming-kang/BUPT_EC",
            "_blank",
            "noopener,noreferrer"
          )
        }
        type="text"
        size="small"
        icon={<GithubIcon />}
        aria-label="GitHub"
      />
    </div>
  );
}

export default Footer;
