import { Button } from "antd";
import { GithubIcon } from "./icons";

function Footer() {
  return (
    <div className="app-footer">
      <span className="text-secondary">© 2026~ ming-kang</span>
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
